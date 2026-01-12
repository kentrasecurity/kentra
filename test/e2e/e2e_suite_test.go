//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/kentrasecurity/kentra/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kentra E2E Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// NODE 1 ONLY: Perform global setup
	fmt.Fprintf(GinkgoWriter, "Setting up the cluster (Node 1)...\n")

	// 1. Install CRDs
	err := utils.RunIgnoringOutput(exec.Command("make", "install"))
	Expect(err).NotTo(HaveOccurred(), "Failed to run make install")

	// 2. Deploy the Operator in the test namespace
	projDir, _ := utils.GetProjectDir()
	ns := "kentra-system"
	utils.RunIgnoringOutput(exec.Command("kubectl", "create", "ns", ns))

	// add annotation to the namespace to mark it as managed by Kentra
	utils.RunIgnoringOutput(exec.Command("kubectl", "annotate", "ns", ns, "managed-by-kentra=true"))

	// Apply the kustomize overlay for testing
	overlay := fmt.Sprintf("%s/config/overlays/test", projDir)
	err = utils.RunIgnoringOutput(exec.Command("kubectl", "apply", "-k", overlay, "-n", ns))
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy operator overlay")

	return nil // Data passed to all nodes if needed
}, func(data []byte) {
	// ALL NODES: Runs after Node 1 finishes the setup block above
	// You can put shared initialization here if necessary
})

var _ = SynchronizedAfterSuite(func() {
	// ALL NODES cleanup
}, func() {
	// NODE 1 ONLY: Final teardown
	fmt.Fprintf(GinkgoWriter, "Tearing down cluster resources (Node 1)...\n")
	utils.RunIgnoringOutput(exec.Command("kubectl", "delete", "ns", "kentra-system"))
})
