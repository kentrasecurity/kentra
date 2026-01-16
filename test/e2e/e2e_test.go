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
		// Entry("OSINT Sherlock", AttackConfig{
		// 	Kind: "osint",
		// 	Name: "osint-sample",
		// 	Time: "10m",
		// 	Label: map[string]string{
		// 		"app":                     "osint",
		// 		"kentra.sh/resource-type": "attack",
		// 		"tool":                    "sherlock",
		// 	},
		// 	Samples: []string{
		// 		"config/samples/assetpools/assetpool-sherlock.yaml",
		// 		"config/samples/attacks/security_v1alpha1_osint_sherlock_with_assets.yaml",
		// 	},
		// }),
		// Entry("Enumeration Rustscan", AttackConfig{
		// 	Kind: "enumeration",
		// 	Name: "rustscan-multi-target",
		// 	Time: "5m",
		// 	Label: map[string]string{
		// 		"app":                     "enumeration",
		// 		"kentra.sh/resource-type": "attack",
		// 		"tool":                    "rustscan",
		// 	},
		// 	Samples: []string{
		// 		"config/samples/targetpools/kttack_v1alpha1_targetpool_rustscan.yaml",
		// 		"config/samples/attacks/kttack_v1alpha1_enumeration_rustscan_multi_target.yaml",
		// 	},
		// }),
		Entry("Enumeration Netcat", AttackConfig{
			Kind: "enumeration",
			Name: "netcat-multi-target",
			Time: "5m",
			Label: map[string]string{
				"app":                     "enumeration",
				"kentra.sh/resource-type": "attack",
				"tool":                    "netcat",
			},
			Samples: []string{
				"config/samples/targetpools/kttack_v1alpha1_targetpool_rustscan.yaml",
				"config/samples/attacks/kttack_v1alpha1_enumeration_with_targetpool.yaml",
			},
		}),
	)
})
