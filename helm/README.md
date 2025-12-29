# KTtack Helm Chart

This Helm chart deploys KTtack, a Kubernetes Security Testing Framework that enables automated security scanning, enumeration, and vulnerability testing through Kubernetes Custom Resource Definitions (CRDs).

## Prerequisites

- Kubernetes 1.11.3+
- Helm 3.0+
- (Optional) Loki instance for centralized logging

## Installation

### Add the Helm repository (if applicable)

```bash
# If hosted in a Helm repository
helm repo add kttack https://charts.kttack.io
helm repo update
```

### Install the chart

```bash
# Install with default values
helm install kttack kttack/kttack --namespace kttack-system --create-namespace

# Install with custom values
helm install kttack kttack/kttack \
  --namespace kttack-system \
  --create-namespace \
  --values custom-values.yaml
```

### Install from local chart

```bash
# From the repository root
helm install kttack ./helm \
  --namespace kttack-system \
  --create-namespace \
  --set loki.host="loki.loki-system.svc.cluster.local" \
  --set loki.port="3100" \
  --set loki.clusterName="my-cluster"
```

## Configuration

The following table lists the configurable parameters of the KTtack chart and their default values.

### Loki Configuration

Configuration for centralized logging using Loki. These credentials are used by the Fluent Bit sidecar to ship logs.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `loki.clusterName` | Kubernetes cluster name for log labeling | `""` (required) |
| `loki.host` | Loki server hostname (e.g., `loki.loki-system.svc.cluster.local`) | `""` (required) |
| `loki.port` | Loki server port | `""` (required, typically `3100`) |
| `loki.user` | Loki authentication username | `""` (required) |
| `loki.password` | Loki authentication password | `""` (required) |
| `loki.tenantId` | Loki tenant ID for multi-tenant setups | `""` (required) |
| `loki.tls` | Enable TLS connection to Loki (`true`/`false`) | `""` (required) |
| `loki.tlsVerify` | Verify Loki TLS certificate (`true`/`false`) | `""` (required) |

### Controller Configuration

Configuration for the KTtack controller manager.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.replicas` | Number of controller replicas | `1` |
| `controller.kubernetesClusterDomain` | Kubernetes cluster domain | `cluster.local` |
| `controller.args` | Controller arguments | `["--metrics-bind-address=:8443", "--leader-elect", "--health-probe-bind-address=:8081"]` |
| `controller.image.repository` | Controller image repository | `ghcr.io/kttack/controller` |
| `controller.image.tag` | Controller image tag (overrides chart appVersion) | `latest` |
| `controller.imagePullSecrets` | Image pull secrets for private registries | `[]` |
| `controller.resources.limits.cpu` | CPU limit | `500m` |
| `controller.resources.limits.memory` | Memory limit | `128Mi` |
| `controller.resources.requests.cpu` | CPU request | `10m` |
| `controller.resources.requests.memory` | Memory request | `64Mi` |
| `controller.containerSecurityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `controller.containerSecurityContext.capabilities.drop` | Dropped capabilities | `["ALL"]` |
| `controller.containerSecurityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |
| `controller.podSecurityContext.runAsNonRoot` | Run as non-root user | `true` |
| `controller.podSecurityContext.seccompProfile.type` | Seccomp profile type | `RuntimeDefault` |
| `controller.serviceAccount.annotations` | Service account annotations | `{}` |

### Service Configuration

Configuration for the metrics service.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.ports` | Service ports configuration | See values.yaml |

## Example Configurations

### Minimal Configuration

```yaml
loki:
  clusterName: "production-cluster"
  host: "loki.monitoring.svc.cluster.local"
  port: "3100"
  user: "kttack"
  password: "supersecret"
  tenantId: "1"
  tls: "false"
  tlsVerify: "false"

controller:
  image:
    tag: "v0.1.0"
```

### Production Configuration with Private Registry

```yaml
loki:
  clusterName: "prod-k8s"
  host: "loki.monitoring.svc.cluster.local"
  port: "3100"
  user: "kttack-prod"
  password: "changeme"
  tenantId: "production"
  tls: "true"
  tlsVerify: "true"

controller:
  replicas: 2
  
  image:
    repository: "myregistry.io/kttack/controller"
    tag: "v0.1.0"
  
  imagePullSecrets:
    - name: registry-credentials
  
  resources:
    limits:
      cpu: 1000m
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
  
  serviceAccount:
    annotations:
      iam.gke.io/gcp-service-account: "kttack@myproject.iam.gserviceaccount.com"

service:
  type: LoadBalancer
```

### Development Configuration

```yaml
loki:
  clusterName: "dev-cluster"
  host: "loki.loki-system.svc.cluster.local"
  port: "3100"
  user: "admin"
  password: "admin"
  tenantId: "1"
  tls: "false"
  tlsVerify: "false"

controller:
  replicas: 1
  
  image:
    repository: "ghcr.io/kttack/controller"
    tag: "latest"
  
  resources:
    limits:
      cpu: 500m
      memory: 128Mi
    requests:
      cpu: 10m
      memory: 64Mi
```

## Installing with Custom Values

Create a `values.yaml` file with your configuration:

```bash
# Create your custom values file
cat > my-values.yaml <<EOF
loki:
  clusterName: "my-cluster"
  host: "loki.monitoring.svc.cluster.local"
  port: "3100"
  user: "kttack"
  password: "mysecretpassword"
  tenantId: "1"
  tls: "false"
  tlsVerify: "false"

controller:
  image:
    tag: "v0.1.0"
EOF

# Install with custom values
helm install kttack ./helm \
  --namespace kttack-system \
  --create-namespace \
  --values my-values.yaml
```

## Upgrading

```bash
# Upgrade to a new version
helm upgrade kttack kttack/kttack \
  --namespace kttack-system \
  --values my-values.yaml

# Or upgrade from local chart
helm upgrade kttack ./helm \
  --namespace kttack-system \
  --values my-values.yaml
```

## Uninstalling

```bash
# Uninstall the chart
helm uninstall kttack --namespace kttack-system

# Note: CRDs are NOT removed by helm uninstall
# To remove CRDs manually:
kubectl delete crd assetpools.kttack.io
kubectl delete crd enumerations.kttack.io
kubectl delete crd livenesses.kttack.io
kubectl delete crd osints.kttack.io
kubectl delete crd securityattacks.kttack.io
kubectl delete crd storagepools.kttack.io
kubectl delete crd targetpools.kttack.io
```

## Custom Resource Definitions (CRDs)

This chart installs the following CRDs:

- `assetpools.kttack.io` - Asset pool management
- `enumerations.kttack.io` - Enumeration operations
- `livenesses.kttack.io` - Liveness checks
- `osints.kttack.io` - OSINT operations
- `securityattacks.kttack.io` - Security attack simulations
- `storagepools.kttack.io` - Storage pool management
- `targetpools.kttack.io` - Target pool management

CRDs are located in the `crds/` directory and are installed automatically by Helm.

## Tool Specifications

KTtack includes pre-configured specifications for various security tools:

- **Enumeration Tools**: nmap, masscan, rustscan, gobuster, feroxbuster, ffuf, dirsearch, nikto
- **DNS Tools**: amass, subfinder, dnsrecon
- **Windows Tools**: enum4linux, CrackMapExec, smbclient
- **Network Tools**: netcat, nbtscan, snmpwalk
- **OSINT Tools**: sherlock, maigret
- **Liveness Tools**: ping

Tool specifications are configured in the `kttack-tool-specs` ConfigMap and cannot be modified via Helm values.

## Fluent Bit Logging

The chart includes a pre-configured Fluent Bit ConfigMap for log aggregation. Fluent Bit sidecars are automatically injected into security testing Jobs to ship logs to Loki.

Configuration is static and includes:

- Log collection from `/logs/job.log`
- Automatic labeling with cluster, namespace, pod, and tool information
- Output to Loki with authentication

## Troubleshooting

### Controller not starting

Check controller logs:
```bash
kubectl logs -n kttack-system deployment/kttack-controller-manager -f
```

### Loki connection issues

Verify Loki credentials:
```bash
kubectl get secret -n kttack-system kttack-loki-credentials -o yaml
```

Test Loki connectivity from within the cluster:
```bash
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://loki.monitoring.svc.cluster.local:3100/ready
```

### CRDs not installing

Check Helm release status:
```bash
helm status kttack -n kttack-system
```

Manually install CRDs if needed:
```bash
kubectl apply -f helm/crds/
```

## Support

For issues, questions, or contributions:
- GitHub: https://github.com/kttack/kttack
- Documentation: https://github.com/kttack/kttack/tree/main/docs

## License

Apache License 2.0 - see LICENSE file for details.
