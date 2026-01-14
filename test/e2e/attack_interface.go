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
	Name, Kind, Time string
	Samples          []string
	Label            map[string]string
}

func RunAttackFlow(ns string, conf AttackConfig) {
	projDir, _ := utils.GetProjectDir()
	prefix := fmt.Sprintf("[%s] ", strings.ToUpper(conf.Kind))
	rate := utils.CalculatePollingInterval(conf.Time)

	// 1. Convertiamo la mappa Label in una stringa selettore per kubectl (es: "app=osint,tool=sherlock")
	var labelParts []string
	for k, v := range conf.Label {
		labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
	}
	labelSelector := strings.Join(labelParts, ",")

	// 2. Apply Resources
	for _, f := range conf.Samples {
		_ = utils.RunIgnoringOutput(exec.Command("kubectl", "apply", "-f", filepath.Join(projDir, f), "-n", ns))
	}

	// 3. Phase 1: Verify Job Creation (Usa il selettore dinamico)
	fmt.Printf("%s⏳ Waiting for Jobs of %s (Selector: %s)...\n", prefix, conf.Name, labelSelector)
	Eventually(func() error {
		// Verifichiamo se esiste almeno un job con quei label
		return utils.RunIgnoringOutput(exec.Command("kubectl", "get", "jobs", "-n", ns, "-l", labelSelector))
	}, "2m", "5s").Should(Succeed(), "At least one Job should be created for: "+conf.Name)
	fmt.Printf("%s✅ Job(s) detected.\n", prefix)

	// 4. Phase 2: Verify Completion of ALL Jobs
	fmt.Printf("%s⏳ Waiting for all Jobs of %s to complete...\n", prefix, conf.Name)

	Eventually(func() error {
		// Otteniamo il conteggio totale dei Job presenti per questo attacco
		countOut, _ := utils.Run(exec.Command("kubectl", "get", "jobs", "-n", ns, "-l", labelSelector, "--no-headers"))
		jobLines := strings.Split(strings.TrimSpace(string(countOut)), "\n")
		totalJobs := len(jobLines)
		if string(countOut) == "" {
			totalJobs = 0
		}

		if totalJobs == 0 {
			return fmt.Errorf("no jobs found with selector: %s", labelSelector)
		}

		// Verifichiamo quanti sono marcati come "Complete"
		// Usiamo un JSONPath robusto che restituisce una lista di status
		cmd := exec.Command("kubectl", "get", "jobs", "-n", ns, "-l", labelSelector, "-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Complete\")].status}")
		out, err := utils.Run(cmd)
		if err != nil {
			return err
		}

		statuses := strings.Fields(string(out))
		completedCount := 0
		for _, s := range statuses {
			if s == "True" {
				completedCount++
			}
		}

		// Se non tutti i job trovati sono completati, ritenta
		if completedCount < totalJobs {
			return fmt.Errorf("waiting for completion: %d/%d jobs finished", completedCount, totalJobs)
		}

		// Controllo rapido per i fallimenti
		failCmd := exec.Command("kubectl", "get", "jobs", "-n", ns, "-l", labelSelector, "-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Failed\")].status}")
		failOut, _ := utils.Run(failCmd)
		if strings.Contains(string(failOut), "True") {
			return fmt.Errorf("one or more jobs FAILED for %s", conf.Name)
		}

		return nil
	}, conf.Time, rate).Should(Succeed(), "All jobs for "+conf.Name+" should complete successfully")

	fmt.Printf("%s🏁 All Jobs for %s finished successfully.\n", prefix, conf.Name)
}
