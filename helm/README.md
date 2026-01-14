# Kentra Helm Chart

This Helm chart deploys Kentra, a Kubernetes Security Testing Framework that enables automated security scanning, enumeration, and vulnerability testing through Kubernetes Custom Resource Definitions (CRDs).

## Prerequisites

- Kubernetes 1.11.3+
- Helm 3.0+
- (Optional) Loki instance for centralized logging

## Installation

### Add the Helm repository (if applicable)

```bash
# If hosted in a Helm repository
helm repo add kentra https://charts.kentra.sh
helm repo update
```

### Install the chart

```bash
# Install with default values
helm install kentra kentra/kentra --namespace kentra-system --create-namespace

# Install with custom values
helm install kentra kentra/kentra \
  --namespace kentra-system \
  --create-namespace \
  --values custom-values.yaml
```

### Install from local chart

```bash
# From the repository root
helm install kentra ./helm \
  --namespace kentra-system \
  --create-namespace \
  --set loki.url="http://loki.loki-system.svc.cluster.local:3100" \
  --set loki.orgId="1"
```

## Configuration

The following table lists the configurable parameters of the Kentra chart and their default values.

### CRD Configuration

Configuration for Custom Resource Definition installation and management.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `crds.install` | Install CRDs with this chart. Set to `false` if CRDs are managed separately or already installed | `true` |

**Important Notes:**
- CRDs are now installed in `templates/crds/` and are automatically upgraded with `helm upgrade`
- If you're using this chart as a subchart, you can disable CRD installation by setting `kentra.crds.install=false`
- CRDs include: `assetpools`, `enumerations`, `exploits`, `livenesses`, `osints`, `securityattacks`, `storagepools`, `targetpools`

### Loki Configuration

Configuration for centralized logging using Loki. These credentials are used by the Fluent Bit sidecar to ship logs.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `loki.url` | Loki server URL (e.g., `http://loki.loki-system.svc.cluster.local:3100`) | `http://loki.loki-system.svc.cluster.local:3100` (required) |
| `loki.orgId` | Loki organization/tenant ID for multi-tenant setups | `""` |
| `loki.auth.username` | Loki authentication username | `""` |
| `loki.auth.password` | Loki authentication password | `""` |

### Controller Configuration

Configuration for the Kentra controller manager.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.replicas` | Number of controller replicas | `1` |
| `controller.kubernetesClusterDomain` | Kubernetes cluster domain | `cluster.local` |
| `controller.args` | Controller arguments | `["--metrics-bind-address=:8443", "--leader-elect", "--health-probe-bind-address=:8081"]` |
| `controller.image.repository` | Controller image repository | `kentrasecurity/docker/controller` |
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
  url: "http://loki.monitoring.svc.cluster.local:3100"
  orgId: "1"
  auth:
    username: "kentra"
    password: "supersecret"

controller:
  image:
    tag: "v0.1.0"
```

### Production Configuration with Private Registry

```yaml
loki:
  url: "https://loki.monitoring.svc.cluster.local:3100"
  orgId: "production"
  auth:
    username: "kentra-prod"
    password: "changeme"

controller:
  replicas: 2

  image:
    repository: "myregistry.io/kentra/controller"
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
      iam.gke.io/gcp-service-account: "kentra@myproject.iam.gserviceaccount.com"

service:
  type: LoadBalancer
```

### Development Configuration

```yaml
loki:
  url: "http://loki.loki-system.svc.cluster.local:3100"
  orgId: "1"
  auth:
    username: "admin"
    password: "admin"

controller:
  replicas: 1

  image:
    repository: "kentrasecurity/docker/controller"
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
  url: "http://loki.monitoring.svc.cluster.local:3100"
  orgId: "1"
  auth:
    username: "kentra"
    password: "mysecretpassword"

controller:
  image:
    tag: "v0.1.0"
EOF

# Install with custom values
helm install kentra ./helm \
  --namespace kentra-system \
  --create-namespace \
  --values my-values.yaml
```

## Upgrading

```bash
# Upgrade to a new version
helm upgrade kentra kentra/kentra \
  --namespace kentra-system \
  --values my-values.yaml

# Or upgrade from local chart
helm upgrade kentra ./helm \
  --namespace kentra-system \
  --values my-values.yaml
```

**Note:** Starting from version 0.3.0, CRDs are automatically upgraded with `helm upgrade`. Previously, CRDs were in the `crds/` directory and required manual updates.

## Uninstalling

```bash
# Uninstall the chart
helm uninstall kentra --namespace kentra-system

# WARNING: CRDs are NOT automatically removed by helm uninstall
# This is intentional to prevent accidental data loss of custom resources

# To remove CRDs and ALL associated custom resources, run:
kubectl delete crd assetpools.kentra.sh
kubectl delete crd enumerations.kentra.sh
kubectl delete crd exploits.kentra.sh
kubectl delete crd livenesses.kentra.sh
kubectl delete crd osints.kentra.sh
kubectl delete crd securityattacks.kentra.sh
kubectl delete crd storagepools.kentra.sh
kubectl delete crd targetpools.kentra.sh
```

## Custom Resource Definitions (CRDs)

This chart installs the following CRDs:

- `assetpools.kentra.sh` - Asset pool management
- `enumerations.kentra.sh` - Enumeration operations
- `exploits.kentra.sh` - Exploit management
- `livenesses.kentra.sh` - Liveness checks
- `osints.kentra.sh` - OSINT operations
- `securityattacks.kentra.sh` - Security attack simulations
- `storagepools.kentra.sh` - Storage pool management
- `targetpools.kentra.sh` - Target pool management

CRDs are located in `templates/crds/` and are automatically installed and upgraded by Helm. You can disable CRD installation by setting `crds.install=false` in your values.

## Tool Specifications

Kentra includes pre-configured specifications for various security tools:

- **Enumeration Tools**: nmap, masscan, rustscan, gobuster, feroxbuster, ffuf, dirsearch, nikto
- **DNS Tools**: amass, subfinder, dnsrecon
- **Windows Tools**: enum4linux, CrackMapExec, smbclient
- **Network Tools**: netcat, nbtscan, snmpwalk
- **OSINT Tools**: sherlock, maigret
- **Liveness Tools**: ping

Tool specifications are configured in the `kentra-tool-specs` ConfigMap and cannot be modified via Helm values.

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
kubectl logs -n kentra-system deployment/kentra-controller-manager -f
```

### Loki connection issues

Verify Loki credentials:
```bash
kubectl get secret -n kentra-system kentra-loki-credentials -o yaml
```

Test Loki connectivity from within the cluster:
```bash
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://loki.monitoring.svc.cluster.local:3100/ready
```

### CRDs not installing or updating

Check Helm release status:
```bash
helm status kentra -n kentra-system
```

Verify CRD installation:
```bash
kubectl get crds | grep kentra.sh
```

If CRDs are not being installed, check that `crds.install` is set to `true`:
```bash
helm get values kentra -n kentra-system
```

Manually apply CRDs if needed:
```bash
kubectl apply -f helm/templates/crds/
```

## Support

For issues, questions, or contributions:
- GitHub: https://github.com/kentra/kentra
- Documentation: https://github.com/kentra/kentra/tree/main/docs

## License

Apache License 2.0 - see LICENSE file for details.
