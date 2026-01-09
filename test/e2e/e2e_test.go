package e2e

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Kentra Attack Operator", func() {
	const ns = "kentra-system"

	// Each Entry here is a separate Spec (test case)
	DescribeTable("Attack Scenarios",
		func(conf AttackConfig) {
			RunAttackFlow(ns, conf)
		},
		Entry("OSINT Sherlock", AttackConfig{
			Kind: "osint",
			Name: "osint-sample",
			Time: "10m",
			Samples: []string{
				"config/samples/assetpools/assetpool-sherlock.yaml",
				"config/samples/attacks/security_v1alpha1_osint_sherlock_with_assets.yaml",
			},
		}),
		Entry("Enumeration Nmap", AttackConfig{
			Kind: "enumeration",
			Name: "nmap-scan-ports",
			Time: "5m",
			Samples: []string{
				"config/samples/targetpools/kttack_v1alpha1_targetpool_nmap_ports.yaml",
				"config/samples/attacks/kttack_v1alpha1_enumeration_nmap_scan_ports.yaml",
			},
		}),
	)
})
