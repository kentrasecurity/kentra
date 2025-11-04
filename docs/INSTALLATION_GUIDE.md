# KTtack Installation Guide

This document provides detailed installation instructions for various scenarios and deployment patterns.

## Table of Contents

1. [Prerequisites Verification](#prerequisites-verification)
2. [Installation Methods](#installation-methods)
3. [Post-Installation Configuration](#post-installation-configuration)
4. [Troubleshooting Installation](#troubleshooting-installation)
5. [Upgrading KTtack](#upgrading-kttack)
6. [Uninstallation](#uninstallation)

## Prerequisites Verification

Before starting the installation, verify all prerequisites are installed and properly configured.

### Check Go Version

```bash
go version
# Expected output: go version go1.24.0 (or higher)
```

If Go is not installed, download from [golang.org](https://golang.org/dl/).

### Check Docker Version

```bash
docker version
# Expected output: Docker version 17.03 or higher
```

For installation instructions, visit [Docker Documentation](https://docs.docker.com/get-docker/).

### Check kubectl Version

```bash
kubectl version --client
# Expected output: GitVersion: v1.11.3 (or higher)
```

For installation, see [Kubernetes Documentation](https://kubernetes.io/docs/tasks/tools/).

### Verify Kubernetes Cluster Access

```bash
# List nodes to verify cluster connectivity
kubectl get nodes

# Verify you have cluster-admin permissions
kubectl auth can-i create clusterrolebinding --as=system:serviceaccount:kttack-system:kttack-controller-manager
```

### Check Container Registry Access

```bash
# For Docker Hub
docker login docker.io

# For GitHub Container Registry
docker login ghcr.io

# For Google Cloud Registry
gcloud auth configure-docker gcr.io
```

## Installation Methods

### Method 1: From Pre-built Release (Recommended for Production)

This is the fastest way to get started with official releases.

#### Step 1: Set Environment Variables

```bash
# Choose your registry and image version
export IMG=ghcr.io/kttack/kttack:v1.0.0  # or docker.io/kttack/kttack:v1.0.0
export KTTACK_NAMESPACE=kttack-system     # namespace for the controller
```

#### Step 2: Create Namespace

```bash
kubectl create namespace ${KTTACK_NAMESPACE}
```

#### Step 3: Install CRDs

```bash
# Download and apply CRD manifests
make install
```

This installs:
- `SecurityAttack` CRD
- `Enumeration` CRD
- `Liveness` CRD

Verify CRDs are installed:

```bash
kubectl get crds | grep kttack.io
# Expected output:
# enumerations.kttack.io                          2025-01-01T10:00:00Z
# livenesses.kttack.io                            2025-01-01T10:00:00Z
# securityattacks.kttack.io                       2025-01-01T10:00:00Z
```

#### Step 4: Deploy the Manager

```bash
# Deploy KTtack controller
make deploy IMG=${IMG}

# Verify manager pod is running
kubectl get pods -n ${KTTACK_NAMESPACE}
kubectl logs -n ${KTTACK_NAMESPACE} -l control-plane=controller-manager -f
```

### Method 2: Build from Source

For development or custom builds.

#### Step 1: Clone Repository

```bash
git clone https://github.com/kttack/kttack.git
cd kttack
```

#### Step 2: Set Up Build Environment

```bash
# Set Go modules
go mod download

# Verify Go version
go version

# Verify project structure
ls -la Makefile cmd/ api/ internal/
```

#### Step 3: Build Binary

```bash
# Build the manager binary for your platform
make build

# Binary will be created at ./bin/manager
./bin/manager --help
```

#### Step 4: Build Docker Image

```bash
# Set your registry
export IMG=your-registry/kttack:v1.0.0

# Build Docker image
make docker-build IMG=${IMG}

# Verify image was created
docker images | grep kttack
```

#### Step 5: Push to Registry

```bash
# Login to registry (if needed)
docker login your-registry

# Push image
make docker-push IMG=${IMG}

# Verify image in registry
docker pull ${IMG}
```

#### Step 6: Install CRDs and Deploy

```bash
# Install CRDs
make install

# Deploy manager
make deploy IMG=${IMG}

# Monitor deployment
kubectl rollout status deployment/kttack-controller-manager -n kttack-system
```

### Method 3: Using Kustomize (Bundle Distribution)

For simplified distribution and offline installation.

#### Step 1: Generate Bundle

```bash
# Set image
export IMG=your-registry/kttack:v1.0.0

# Build installer bundle
make build-installer IMG=${IMG}

# Verify bundle was created
ls -la dist/install.yaml
```

#### Step 2: Deploy from Bundle

For users installing from the bundle:

```bash
# Direct installation from URL
kubectl apply -f https://path-to-bundle/install.yaml

# Or local installation
kubectl apply -f dist/install.yaml

# Verify installation
kubectl get pods -n kttack-system
kubectl get crds | grep kttack.io
```

The bundle includes:
- CRDs
- ServiceAccount
- Roles and RoleBindings
- Manager Deployment
- ConfigMaps and Secrets

### Method 4: Using Helm Chart (Optional)

For advanced users preferring Helm.

#### Step 1: Generate Helm Chart

```bash
# Install Helm plugin for kubebuilder
kubebuilder edit --plugins=helm/v1-alpha

# Chart will be generated at dist/chart/
ls -la dist/chart/
```

#### Step 2: Customize Helm Values

```bash
# Edit values
vi dist/chart/values.yaml

# Key values:
# - image.repository: Container image repository
# - image.tag: Container image tag
# - manager.replicas: Number of manager replicas
# - resources.limits: Resource limits
```

#### Step 3: Install with Helm

```bash
# Add Helm repository (if using remote chart)
helm repo add kttack https://charts.kttack.io

# Install
helm install kttack kttack/kttack \
  --namespace kttack-system \
  --create-namespace \
  -f dist/chart/values.yaml

# Verify installation
helm list -n kttack-system
kubectl get pods -n kttack-system
```

#### Step 4: Upgrade with Helm

```bash
# Update chart repository
helm repo update

# Upgrade
helm upgrade kttack kttack/kttack \
  --namespace kttack-system
```

## Post-Installation Configuration

### Configure Tool Specifications

Tool specifications define which security tools are available and their configurations.

#### Step 1: Create Tool Specs ConfigMap

Create a file `tool-specs.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tool-specs
  namespace: kttack-system
data:
  tools.yaml: |
    tools:
      nmap:
        image: "nmap/nmap:latest"
        default_args:
          - "-sV"
          - "-A"
        timeout: 3600
        resource_limits:
          cpu: "500m"
          memory: "256Mi"
        
      nikto:
        image: "docker.io/securecodebox/nikto:latest"
        default_args:
          - "-Format json"
        timeout: 1800
        resource_limits:
          cpu: "250m"
          memory: "128Mi"
        
      feroxbuster:
        image: "docker.io/epi052/feroxbuster:latest"
        default_args:
          - "-s 200,204,301,302,307,401,403,405,500"
        timeout: 2400
        resource_limits:
          cpu: "500m"
          memory: "256Mi"
```

#### Step 2: Apply ConfigMap

```bash
kubectl apply -f tool-specs.yaml

# Verify ConfigMap
kubectl get cm -n kttack-system tool-specs
kubectl get cm -n kttack-system tool-specs -o yaml
```

### Configure Fluent Bit Logging (Optional)

For centralized log aggregation using Loki and Fluent Bit.

#### Step 1: Create Loki Credentials Secret

Create file `loki-secret.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: loki-credentials
  namespace: kttack-system
type: Opaque
stringData:
  loki-host: "loki.example.com"
  loki-port: "443"
  loki-tls: "true"
  loki-tls-verify: "false"
  loki-tenant-id: "1"
  loki-user: "your-user"
  loki-password: "your-password"
  cluster-name: "my-cluster"
```

#### Step 2: Apply Secret

```bash
# Create from file
kubectl apply -f loki-secret.yaml

# Or create from command line
kubectl create secret generic loki-credentials \
  -n kttack-system \
  --from-literal=loki-host=loki.example.com \
  --from-literal=loki-port=443 \
  --from-literal=loki-tls=true \
  --from-literal=loki-user=username \
  --from-literal=loki-password=password

# Verify secret
kubectl get secret -n kttack-system loki-credentials
```

#### Step 3: Create Fluent Bit ConfigMap

Create file `fluent-bit-config.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
  namespace: kttack-system
data:
  fluent-bit.conf: |
    [SERVICE]
        Flush        5
        Daemon       Off
        Log_Level    info
        Parsers_File parsers.conf

    [INPUT]
        Name              tail
        Path              /logs/*.log
        Read_from_Head    true
        Refresh_Interval  5
        Tag               kttack.job.*

    [FILTER]
        Name    modify
        Match   *
        Add     cluster ${CLUSTER_NAME}
        Add     component job
        Add     app kttack

    [OUTPUT]
        Name   loki
        Match  *
        host   ${LOKI_HOST}
        port   ${LOKI_PORT}
        tls    ${LOKI_TLS}
        tls.verify ${LOKI_TLS_VERIFY}
        tenant_id ${LOKI_TENANT_ID}
        http_user ${LOKI_USER}
        http_passwd ${LOKI_PASSWORD}
```

#### Step 4: Apply ConfigMap

```bash
kubectl apply -f fluent-bit-config.yaml

# Verify ConfigMap
kubectl get cm -n kttack-system fluent-bit-config
```

### Verify Post-Installation Setup

```bash
# Check all components
echo "=== Namespace ==="
kubectl get ns kttack-system

echo "=== CRDs ==="
kubectl get crds | grep kttack.io

echo "=== Manager Pod ==="
kubectl get pods -n kttack-system

echo "=== ServiceAccount ==="
kubectl get sa -n kttack-system

echo "=== Roles ==="
kubectl get roles -n kttack-system

echo "=== ConfigMaps ==="
kubectl get cm -n kttack-system

echo "=== Secrets ==="
kubectl get secrets -n kttack-system
```

## Troubleshooting Installation

### Issue: Manager Pod in CrashLoopBackOff

```bash
# Check pod status
kubectl get pods -n kttack-system
kubectl describe pod -n kttack-system <pod-name>

# View logs
kubectl logs -n kttack-system <pod-name>

# Common causes:
# - Missing tool-specs ConfigMap
# - Invalid RBAC permissions
# - Image pull errors
```

### Issue: CRDs Not Installed

```bash
# Verify CRDs exist
kubectl get crd | grep kttack.io

# If missing, reinstall
make install

# Check for errors
kubectl get crd -o yaml | grep -A 5 "kttack.io"
```

### Issue: Image Pull Errors

```bash
# Verify image exists and is accessible
docker pull ${IMG}

# Check image pull secrets in cluster
kubectl get secrets -n kttack-system

# Add secret if needed
kubectl create secret docker-registry regcred \
  --docker-server=<registry> \
  --docker-username=<username> \
  --docker-password=<password> \
  -n kttack-system
```

### Issue: RBAC Errors

```bash
# Verify ServiceAccount exists
kubectl get sa -n kttack-system

# Check role bindings
kubectl get rolebinding -n kttack-system
kubectl get clusterrolebinding | grep kttack

# Verify permissions
kubectl auth can-i create securityattacks \
  --as=system:serviceaccount:kttack-system:kttack-controller-manager
```

## Upgrading KTtack

### Upgrade Procedure

```bash
# 1. Check current version
kubectl get deployment -n kttack-system kttack-controller-manager -o yaml | grep image

# 2. Set new image version
export IMG=your-registry/kttack:v1.1.0

# 3. Pull new image
docker pull ${IMG}

# 4. Update deployment
make deploy IMG=${IMG}

# 5. Monitor rollout
kubectl rollout status deployment/kttack-controller-manager -n kttack-system

# 6. Verify new version
kubectl get pods -n kttack-system
kubectl logs -n kttack-system -l control-plane=controller-manager | tail -20
```

### Rollback Procedure

```bash
# If upgrade fails, rollback to previous version
kubectl rollout undo deployment/kttack-controller-manager -n kttack-system

# Verify rollback
kubectl rollout status deployment/kttack-controller-manager -n kttack-system
```

## Uninstallation

### Complete Uninstallation

```bash
# 1. Delete all SecurityAttack/Enumeration/Liveness resources
kubectl delete securityattacks --all --all-namespaces
kubectl delete enumerations --all --all-namespaces
kubectl delete livenesses --all --all-namespaces

# 2. Wait for related Jobs/CronJobs to complete or be deleted
kubectl delete jobs --all --all-namespaces -l managed-by=kttack
kubectl delete cronjobs --all --all-namespaces -l managed-by=kttack

# 3. Undeploy the manager
make undeploy

# 4. Remove CRDs
make uninstall

# 5. Delete namespace
kubectl delete namespace kttack-system

# 6. Verify complete removal
kubectl get crds | grep kttack.io  # Should return empty
kubectl get ns | grep kttack       # Should return empty
```

### Partial Uninstallation

To keep CRDs but remove the manager:

```bash
# Remove only the manager deployment
make undeploy

# Namespace and CRDs remain for potential re-installation
```

## Verification Checklist

After installation, verify:

- [ ] CRDs installed: `kubectl get crds | grep kttack.io`
- [ ] Manager pod running: `kubectl get pods -n kttack-system`
- [ ] ServiceAccount exists: `kubectl get sa -n kttack-system`
- [ ] RBAC configured: `kubectl get rolebinding -n kttack-system`
- [ ] Webhook ready: `kubectl get validatingwebhookconfigurations`
- [ ] Metrics accessible: `kubectl port-forward -n kttack-system svc/kttack-controller-manager-metrics-service 8080:8080`
- [ ] Tool specs configured: `kubectl get cm -n kttack-system tool-specs`
- [ ] (Optional) Loki credentials: `kubectl get secret -n kttack-system loki-credentials`

## Next Steps

After successful installation:

1. Read [Architecture Guide](./ARCHITECTURE.md) to understand system design
2. Review [Fluent Bit Documentation](./FLUENT_BIT_SIDECAR.md) for logging setup
3. Create your first SecurityAttack: see examples in `config/samples/`
4. Configure tool specifications in `config/default/tool-specs.yaml`
5. Set up monitoring and alerting for your security operations
