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
		return utils.RunIgnoringOutput(exec.Command("kubectl", "get", "jobs", "-n", ns, "-l", "kentra.sh/"+strings.ToLower(conf.Kind)+"-name="+conf.Name))
	}, "2m", "5s").Should(Succeed(), "Job should be created with name: "+conf.Name)
	fmt.Printf("%s✅ Job/%s detected.\n", prefix, conf.Name)

	// 3. Phase 2: Verify Completion of ALL Jobs
	fmt.Printf("%s⏳ Waiting for all Jobs of %s to complete...\n", prefix, conf.Name)

	labelSelector := fmt.Sprintf("kentra.sh/%s-name=%s", strings.ToLower(conf.Kind), conf.Name)

	Eventually(func() error {
		// 1. Ottieni la lista dei Job tramite label
		// jsonpath restituisce lo stato della condizione 'Complete' per ogni job trovato, separato da spazio
		cmd := exec.Command("kubectl", "get", "jobs", "-n", ns, "-l", labelSelector, "-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Complete\")].status}")
		out, err := utils.Run(cmd)
		if err != nil {
			return err
		}

		// Dividiamo l'output (es: "True True" se ci sono due job completati)
		statuses := strings.Fields(string(out))

		// Controlliamo quanti job ci aspettiamo (opzionale, ma consigliato)
		// Se lo YAML ha 2 gruppi, dobbiamo trovare 2 stati "True"
		if len(statuses) == 0 {
			return fmt.Errorf("no jobs found yet")
		}

		for _, status := range statuses {
			if status != "True" {
				return fmt.Errorf("one or more jobs are not completed yet (status: %s)", status)
			}
		}

		// 2. Controllo Fallimenti (opzionale ma utile)
		failCmd := exec.Command("kubectl", "get", "jobs", "-n", ns, "-l", labelSelector, "-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Failed\")].status}")
		failOut, _ := utils.Run(failCmd)
		if strings.Contains(string(failOut), "True") {
			return fmt.Errorf("one or more jobs FAILED")
		}

		return nil
	}, conf.Time, rate).Should(Succeed(), "All jobs for "+conf.Name+" should complete successfully")
}
