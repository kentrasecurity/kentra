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
			Name: "sherlock-sample",
			Time: "10m",
			Label: map[string]string{
				"app":                     "osint",
				"kentra.sh/resource-type": "attack",
				"tool":                    "sherlock",
			},
			Samples: []string{
				"examples/attacks/osint/",
			},
		}),
		Entry("OSINT Trufflehog", AttackConfig{
			Kind: "osint",
			Name: "trufflehog-sample",
			Time: "10m",
			Label: map[string]string{
				"app":                     "osint",
				"kentra.sh/resource-type": "attack",
				"tool":                    "trufflehog",
			},
			Samples: []string{
				"examples/attacks/osint/",
			},
		}),
		Entry("ENUMERATION Rustscan", AttackConfig{
			Kind: "enumeration",
			Name: "rustscan-multi-test",
			Time: "5m",
			Label: map[string]string{
				"app":                     "enumeration",
				"kentra.sh/resource-type": "attack",
				"tool":                    "rustscan",
			},
			Samples: []string{
				"examples/attacks/enumeration/",
			},
		}),
		Entry("Enumeration Netcat", AttackConfig{
			Kind: "enumeration",
			Name: "netcat-banner-grab",
			Time: "5m",
			Label: map[string]string{
				"app":                     "enumeration",
				"kentra.sh/resource-type": "attack",
				"tool":                    "netcat",
			},
			Samples: []string{
				"examples/attacks/enumeration/",
			},
		}),
		Entry("Enumeration Nmap", AttackConfig{
			Kind: "enumeration",
			Name: "nmap-scan-example-2",
			Time: "5m",
			Label: map[string]string{
				"app":                     "enumeration",
				"kentra.sh/resource-type": "attack",
				"tool":                    "nmap",
			},
			Samples: []string{
				"examples/attacks/enumeration/nmap.yaml",
			},
		}),
		Entry("Exploit", AttackConfig{
			Kind: "exploit",
			Name: "metasploit-exploit",
			Time: "10m",
			Label: map[string]string{
				"app":                     "exploit",
				"kentra.sh/resource-type": "attack",
				"tool":                    "metasploit",
			},
			Samples: []string{
				"examples/attacks/exploit/",
			},
		}),
		Entry("Liveness Ping", AttackConfig{
			Kind: "liveness",
			Name: "ping",
			Time: "5m",
			Label: map[string]string{
				"app":                     "liveness",
				"kentra.sh/resource-type": "attack",
				"tool":                    "ping",
			},
			Samples: []string{
				"examples/attacks/liveness/ping.yaml",
			},
		}),
	)
})
