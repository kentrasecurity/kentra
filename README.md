# KTtack - Kubernetes Security Testing Framework

A comprehensive Kubernetes Operator for orchestrating and managing security testing operations within Kubernetes clusters. KTtack enables automated security scanning, enumeration, vulnerability testing, and security attack simulations through Kubernetes Custom Resource Definitions (CRDs).

## Overview

KTtack provides a declarative way to define and execute security operations as native Kubernetes resources. Instead of manually managing security testing tools and scripts, you define your security tests as YAML manifests and let KTtack's Kubernetes Operator handle orchestration, scheduling, logging, and resource management.

### Key Features

- **Custom Resource Definitions (CRDs)**: Define security tests declaratively using Kubernetes-native resources
  - `SecurityAttack`: Execute security testing operations
  - `Enumeration`: Network and service enumeration
  - `Liveness`: System health and availability checks

- **Flexible Tool Integration**: Support for popular security tools (Nmap, Nikto, Feroxbuster, etc.)
- **Periodic Execution**: Schedule security tests using standard Kubernetes CronJob syntax
- **Centralized Logging**: Integrated Fluent Bit sidecar for log aggregation to Loki
- **Multi-tenant Support**: Namespace-aware resource management
- **Comprehensive Monitoring**: Prometheus metrics and health checks

## Documentation

Complete documentation is available in the [docs directory](./docs/):

- **[Documentation Index](./docs/INDEX.md)** - Complete guide to all documentation resources
- **[Getting Started](./docs/GETTING_STARTED.md)** - Quick start guide to create your first security test (5 minutes)
- **[Installation Guide](./docs/INSTALLATION_GUIDE.md)** - Detailed installation instructions for all deployment methods
- **[Architecture Guide](./docs/ARCHITECTURE.md)** - Comprehensive overview of system design, components, and interactions
- **[API Reference](./docs/API_REFERENCE.md)** - Complete CRD specification and field reference
- **[Fluent Bit Sidecar Documentation](./docs/FLUENT_BIT_SIDECAR.md)** - Setup and configuration for centralized logging

## Quick Start

For detailed installation instructions, refer to the [Installation Guide](./docs/INSTALLATION_GUIDE.md). Below is a quick overview of installation steps.

## Getting Started

### Prerequisites

- **Go**: v1.24.0 or higher
- **Docker**: v17.03 or higher
- **kubectl**: v1.11.3 or higher
- **Kubernetes Cluster**: v1.11.3 or higher with cluster-admin access

### Installation Methods

Choose one of the following installation methods based on your needs:

#### Method 1: Quick Start with Pre-built Image

If you have a pre-built Docker image available in a registry:

```bash
# Set your image registry and tag
export IMG=your-registry/kttack:v1.0.0

# Install CRDs
make install

# Deploy the manager
make deploy IMG=$IMG
```

#### Method 2: Build and Deploy from Source

Build the controller from source code:

```bash
# Clone the repository
git clone https://github.com/kttack/kttack.git
cd kttack

# Build the Docker image
make docker-build IMG=your-registry/kttack:v1.0.0

# Push to your registry (ensure you have push permissions)
make docker-push IMG=your-registry/kttack:v1.0.0

# Install CRDs into the cluster
make install

# Deploy the manager
make deploy IMG=your-registry/kttack:v1.0.0
```

**Note**: Replace `your-registry` with your actual Docker registry URL (e.g., `docker.io/myorg`, `ghcr.io/myorg`, `gcr.io/myproject`).

#### Method 3: Using Kustomize (Bundle Distribution)

Create a bundled installation file:

```bash
# Build the installer bundle
make build-installer IMG=your-registry/kttack:v1.0.0
```

This generates an `install.yaml` file in the `dist/` directory containing all resources. Users can then deploy with:

```bash
# Install from bundle
kubectl apply -f https://path-to-your-bundle/install.yaml
```

#### Method 4: Using Helm Chart

Helm chart support is available for simplified deployment. Refer to the Helm values documentation for configuration options.

### Deployment Verification

Verify the deployment was successful:

```bash
# Check if the manager pod is running
kubectl get pods -n kttack-system

# Check CRDs are installed
kubectl get crds | grep kttack.io

# View controller logs
kubectl logs -n kttack-system deployment/kttack-controller-manager -f
```

### Configure Tool Specifications (Optional)

KTtack uses a ConfigMap to define tool specifications:

```bash
kubectl apply -f config/default/tool-specs.yaml
```

### Configure Logging to Loki (Optional)

For centralized logging with Fluent Bit and Loki:

```bash
# Apply Loki credentials secret
kubectl apply -f config/default/loki-secret.yaml

# Apply Fluent Bit configuration
kubectl apply -f config/default/fluent-bit-config.yaml
```

See [Fluent Bit Sidecar Documentation](./docs/FLUENT_BIT_SIDECAR.md) for detailed configuration.

## Usage Examples

### Create a One-Time Security Test

```bash
kubectl apply -f config/samples/kttack_v1alpha1_securityattack.yaml
```

### Create a Periodic Enumeration

```bash
kubectl apply -f config/samples/kttack_v1alpha1_enumeration.yaml
```

### Monitor Running Tests

```bash
# List all security attacks
kubectl get securityattacks -A

# Watch a specific attack
kubectl describe securityattack <name> -n <namespace>

# View attack logs
kubectl logs -n <namespace> -l job-name=<attack-name>
```

## First Test (5-minute Quick Start)

To create your first security test, follow the [Getting Started Guide](./docs/GETTING_STARTED.md).

Quick example:

```yaml
apiVersion: kttack.io/v1alpha1
kind: SecurityAttack
metadata:
  name: my-first-scan
  namespace: security-testing
spec:
  attackType: Enumeration
  target: "192.168.1.0/24"
  tool: nmap
  periodic: false
  debug: true
  args:
    - "-sV"
```

## Uninstallation

### Remove Security Test Resources

Delete all custom resources:

```bash
kubectl delete -k config/samples/
```

### Remove CRDs and Controller

Uninstall the controller and CRDs:

```bash
# Undeploy the manager
make undeploy

# Remove CRDs
make uninstall
```

## Development

### Building from Source

```bash
# Generate manifests (CRDs, RBAC)
make manifests

# Generate Go code
make generate

# Run tests
make test

# Build binary
make build
```

### Testing

```bash
# Run unit tests
make test

# Run end-to-end tests (requires Kind cluster)
make setup-test-e2e
```

### Code Style

```bash
# Format code
make fmt

# Run static analysis
make vet
```

## Project Distribution

### Publishing to Container Registry

Ensure your image is published to a registry accessible from your Kubernetes cluster:

```bash
make docker-build docker-push IMG=<registry>/kttack:tag
```

### Creating Release Bundles

Generate a complete installation bundle:

```bash
make build-installer IMG=<registry>/kttack:tag
```

Users can then deploy using:

```bash
kubectl apply -f dist/install.yaml
```

### Helm Chart Distribution

Build the Helm chart for distribution:

```bash
kubebuilder edit --plugins=helm/v1-alpha
```

The generated chart will be available in the `dist/chart` directory.

## Contributing

Contributions are welcome! Please ensure that any changes:

1. Follow the existing code style and patterns
2. Include appropriate tests
3. Update documentation as needed
4. Pass all existing tests and linters

## Support

For issues, questions, or contributions, please visit the [GitHub Repository](https://github.com/kttack/kttack).

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
