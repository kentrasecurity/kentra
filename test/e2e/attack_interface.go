package e2e

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kentrasecurity/kentra/test/utils"
	. "github.com/onsi/gomega"
)

type AttackConfig struct {
	Name, Kind, Label, Time string
	Samples                 []string
}

func RunAttackFlow(ns string, conf AttackConfig) {
	projDir, _ := utils.GetProjectDir()
	prefix := fmt.Sprintf("[%s] ", strings.ToUpper(conf.Kind))
	rate := utils.CalculatePollingInterval(conf.Time) // calculate rate based on time

	// 1. Apply Resources
	for _, f := range conf.Samples {
		_ = utils.RunIgnoringOutput(exec.Command("kubectl", "apply", "-f", filepath.Join(projDir, f), "-n", ns))
	}

	// 2. Phase 1: Verify Job Creation (Match by Name)
	fmt.Printf("%s⏳ Waiting for Job/%s...\n", prefix, conf.Name)
	Eventually(func() error {
		return utils.RunIgnoringOutput(exec.Command("kubectl", "get", "job", conf.Name, "-n", ns))
	}, "2m", "5s").Should(Succeed(), "Job should be created with name: "+conf.Name)
	fmt.Printf("%s✅ Job/%s detected.\n", prefix, conf.Name)

	// 3. Phase 2: Verify Completion
	fmt.Printf("%s⏳ Waiting for Job/%s to complete...\n", prefix, conf.Name)
	Eventually(func() error {
		// Check Success
		out, _ := utils.Run(exec.Command("kubectl", "get", "job", conf.Name, "-n", ns, "-o", "jsonpath={.status.conditions[?(@.type==\"Complete\")].status}"))
		if strings.Contains(string(out), "True") {
			return nil
		}

		// Check Failure
		fail, _ := utils.Run(exec.Command("kubectl", "get", "job", conf.Name, "-n", ns, "-o", "jsonpath={.status.conditions[?(@.type==\"Failed\")].status}"))
		if strings.Contains(string(fail), "True") {
			// Fetch logs for the specific job using its selector
			pod, _ := utils.Run(exec.Command("kubectl", "get", "pods", "-l", "job-name="+conf.Name, "-n", ns, "-o", "jsonpath={.items[0].metadata.name}"))
			logs, _ := utils.Run(exec.Command("kubectl", "logs", strings.TrimSpace(string(pod)), "-n", ns))
			return fmt.Errorf("JOB FAILED! LOGS:\n%s", string(logs))
		}

		return fmt.Errorf("job still running...")
	}, conf.Time, rate).Should(Succeed())

	fmt.Printf("%s🏁 Job/%s finished successfully.\n", prefix, conf.Name)
}
