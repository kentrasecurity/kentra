# Kentra E2E Testing Guide

This guide explains how our end-to-end (E2E) testing works and how you can add your own attack scenarios to the suite.
## The Test Flow

When you run `make test-e2e-kind`, this happens in this specific order:

1. Build & Cluster:

    - A Kind (Kubernetes-in-Docker) cluster is spun up.

    - We build the Operator Docker image (controller:test) locally. The name of this image is specified in the command `test-e2e-kind` of the Makefile.

    - The image is "loaded" into the Kind nodes so they can pull it without an external registry.

2. Setup:

    - We run `make install` to push our CRDs to the cluster.

    - We deploy the operator using a test kustomize overlay into the kentra-system namespace (at `config/overlays/test/kustomization.yaml`).

3. Execution:

    - Ginkgo runs the "Attack Scenarios" in parallel.

4. Cleanup:

    - Once finished (or if it fails), the cluster is deleted.

## Parallelization

We use Ginkgo V2 to run tests in parallel using the -p flag.

- `SynchronizedBeforeSuite` at `test/e2e/e2e_suite_test.go`: This ensures that Node 1 handles the "heavy lifting" (like make install) while other nodes wait.

- Independent Nodes: Each worker (Node) picks up an Entry from our table and runs it. This cuts down our test time significantly—instead of waiting for OSINT then Enumeration, they run at the same time. You can set the level of Ginkgo parallelization with the `--procs` flag in the Makefile

*Note: in Ginkgo a "Node" isn't a Kubernetes worker node or a physical computer. It is a parallel process (or worker) that Ginkgo spawns to run your tests.*

## Contribution: How to add a new Test

1. Create your Attack Sample

Add your YAML manifests to `config/samples/` (e.g. Target/Asset Pool and the Attack CR itself)

2. Add the Entry

In `test/e2e/e2e_test.go` add a new **Entry** to the **DescribeTable**:

```Go
Entry("My_New_Attack", AttackConfig{
    Kind:    "my-kind",      // The CRD kind
    Name:    "my-sample",    // metadata.name of your attack
    Time:    "5m",           // Max time to wait for completion
    Samples: []string{
        "config/samples/my_pool.yaml",
        "config/samples/my_attack.yaml",
    },
}),
```

3. Run `make test-e2e-kind`