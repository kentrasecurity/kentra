<h1 align="center">Kentra</h1>

<h3 align="center">
  <a name="readme-top"></a>
  <img
    src="img/logo.svg"
    height="250"
  >
</h3>
<div align="center">

<p align="center">
  A Kubernetes offensive security framework for orchestrating penetration testing, red teaming operations, and large-scale reproducible security scans both inside and outside your cluster.
</p>

<div align="center">
  <a href="https://github.com/kentrasecurity/kentra/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/kentrasecurity/kentra" alt="License">
  </a>
  <a href="https://github.com/kentrasecurity/kentra/releases">
    <img src="https://img.shields.io/github/v/release/kentrasecurity/kentra" alt="Release">
  </a>
</div>

<div align="center">
  <a href="https://github.com/orgs/kentrasecurity/packages/container/package/helm/kentra">
    <img src="https://img.shields.io/badge/ghcr.io-kentra-blue" alt="GitHub Container Registry">
  </a>
  <a href="https://kubernetes.io/">
    <img src="https://img.shields.io/badge/kubernetes-ready-326CE5?logo=kubernetes&logoColor=white" alt="Kubernetes">
  </a>
  <a href="https://github.com/kentrasecurity/kentra/tree/main/helm">
    <img src="https://img.shields.io/badge/helm-chart-0F1689?logo=helm&logoColor=white" alt="Helm Chart">
  </a>
</div>

<br>

<p align="center">
  <a href="#overview">Overview</a> •
  <a href="#-installation-methods">Installation</a> •
  <a href="#key-features">Features</a> •
  <a href="./docs/GETTING_STARTED.md">Quick Start</a> •
  <a href="./ToDo.md">Todo</a> •
  <a href="#contributing">Contributing</a>
</p>

<br>
</div>

## Overview

Kentra provides a declarative way to define and execute security operations as native Kubernetes resources. Instead of manually managing security testing tools and scripts, you define your security tests as YAML manifests and let Kentra's Kubernetes Operator handle orchestration, scheduling, logging, and resource management.

## Demo
This is a view-only demo. 

To explore all Kentra features, please spin up the project and have fun :)
#### Kentra URL: [https://demo.kentrasecurity.sh](https://demo.kentrasecurity.sh)

## Prerequisites
A Kubernetes cluster

## Installation
### Helm Chart

Helm chart support is available for simplified deployment. Refer to the Helm values documentation for configuration options.

```bash
helm install kentra-platform \
  oci://ghcr.io/kentrasecurity/helm/kentra-platform \
  --version 0.4.0 \
  --namespace kentra-system \
  --create-namespace \
  -f values.yaml
```

To uninstall it 
```bash
helm uninstall kentra-platform -n kentra-system
```


### Kustomize
This will use Kustomize to install Kentra via [kustomization.yaml](kentra/config/default/kustomization.yaml). The default namespace is `kentra-system`
```bash
kubectl apply -k config/default
```
To uninstall it 
```bash
kubectl delete -k config/default
```
### Verify the Deployment

```bash
# Check if the manager pod is running
kubectl get pods -n kentra-system

# Check CRDs are installed
kubectl get crds | grep kentra.sh

# View controller logs
kubectl logs -n kentra-system deployment/kentra-controller-manager -f
```

## Quick Start

Quick example:

```yaml
apiVersion: kentra.sh/v1alpha1
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

## Configure Tool Specifications
Kentra uses the ConfigMap [tool-specs.yaml](config/default/tool-specs.yaml) to define tool specifications. When modified, apply it again with

```bash
kubectl apply -f config/default/kentra-tool-specs.yaml
```

To specify a new tool, use the following fields:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `type` | string | The operation type the tool performs, used for greppable purposes | `"enumeration"`, `"exploitation"`, `"scanning"` |
| `category` | string | The category or domain of the tool, used for logic separation | `"network"`, `"web"`, `"vulnerability"` |
| `image` | string | Docker image URI for the tool container | `"instrumentisto/nmap:latest"` |
| `commandTemplate` | string | Command execution template with placeholders | `"nmap {{.Args}} -p {{.Target.port}} {{.Target.endpoint}}"` |
| `endpointSeparator` | string (Optional) | Delimiter for multiple endpoints/targets (if supported by the tool) | `" "` (space), `","` (comma) |
| `portSeparator` | string (Optional) | Delimiter for multiple ports (if supported by the tool)| `","` |
| `capabilities` | object (Optional)| Linux capabilities required for the container | [See all capabilities example](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container0) |

**Example configuration:** [nmap](https://nmap.org/)

Nmap is a standard and well knonw tool that support both separators for targets and ports, [as specified in the documentation](https://nmap.org/book/port-scanning-options.html).

```yaml
nmap:
  type: "enumeration"
  category: "network"
  image: "instrumentisto/nmap:latest"
  commandTemplate: "nmap {{.Args}} -p {{.Target.port}} {{.Target.endpoint}}"
  endpointSeparator: " "
  portSeparator: ","
  capabilities:
    add:
      - NET_RAW
```

> [!IMPORTANT]
>
> This example of toolspec's nmap configuration will rely on `endpointSeparator` and `portSeparator` to create the full nmap command. If no separator is specified, or if the tool doesn't support the separator, **Kentra will create a command for each entry in the target and in the port section.**

See [EXAMPLES.md](docs/EXAMPLES.md) for full examples and configurations


## Configure Logging

For centralized logging with Fluent Bit and Loki view [LOGGING.md](docs/LOGGING.md)

## Usage Examples

### Create a One-Time Security Test

```bash
kubectl apply -f config/samples/kentra_v1alpha1_securityattack.yaml
```

### Create a Periodic Enumeration

```bash
kubectl apply -f config/samples/kentra_v1alpha1_enumeration.yaml
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

## Development & Building from Source

To see development and compilation process view the [development documentation](docs/DEVELOPMENT.md)

## Disclaimers
**User Responsibility & Legal Notice**
> [!CAUTION]
>
> You are required to secure clear, written permission from the system owner before using Kentra on any target. 
Kentra Security and its contributors disclaim all responsibility for any harm, damages, losses, or legal repercussions arising from the use of this project. This includes, but is not limited to, unauthorized access, data breaches, system disruption, or criminal charges. By using this tool, you acknowledge that you are solely accountable for your actions and any resulting consequences..

## Contributing

Contributions are welcome! Please ensure that any changes:

0. Fork the project and make your changes
1. Follow the existing code style and patterns
2. Include appropriate tests
3. Update documentation as needed
4. Pass all existing tests and linters
5. Open a Pull Request
6. Merged :)

## Support

For issues, questions, or contributions, please visit the [GitHub Repository](https://github.com/kentrasecurity/kentra).
