# KTtack Architecture

This document provides a comprehensive overview of the KTtack Kubernetes Operator architecture, including its components, design patterns, and interactions.

## Table of Contents

1. [High-Level Architecture](#high-level-architecture)
2. [Core Components](#core-components)
3. [Custom Resource Definitions (CRDs)](#custom-resource-definitions-crds)
4. [Controller Reconciliation Flow](#controller-reconciliation-flow)
5. [Job and CronJob Management](#job-and-cronjob-management)
6. [Tool Integration](#tool-integration)
7. [Logging and Monitoring](#logging-and-monitoring)
8. [Security and RBAC](#security-and-rbac)

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │         KTtack System Namespace                    │    │
│  │  (kttack-system)                                   │    │
│  │                                                    │    │
│  │  ┌──────────────────────────────────────────┐    │    │
│  │  │  Manager Pod (Controller Runtime)        │    │    │
│  │  │  - SecurityAttackReconciler              │    │    │
│  │  │  - EnumerationReconciler                 │    │    │
│  │  │  - LivenessReconciler                    │    │    │
│  │  │  - ToolSpecManager                       │    │    │
│  │  │  - ToolsConfigurator                     │    │    │
│  │  └──────────────────────────────────────────┘    │    │
│  │                                                    │    │
│  │  ┌──────────────────────────────────────────┐    │    │
│  │  │  ConfigMaps & Secrets                    │    │    │
│  │  │  - kttack-tool-specs (tool definitions)         │    │    │
│  │  │  - fluent-bit-config (logging)           │    │    │
│  │  │  - loki-credentials (log destination)    │    │    │
│  │  └──────────────────────────────────────────┘    │    │
│  │                                                    │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │    User Namespaces (security-testing, etc.)       │    │
│  │                                                    │    │
│  │  SecurityAttack / Enumeration / Liveness CRs     │    │
│  │         ↓                                          │    │
│  │  ┌───────────────────────────────────────┐        │    │
│  │  │  Jobs & CronJobs (Batch API)          │        │    │
│  │  │  - One-time Job (one-off attacks)     │        │    │
│  │  │  - CronJob (periodic scanning)        │        │    │
│  │  └───────────────────────────────────────┘        │    │
│  │         ↓                                          │    │
│  │  ┌───────────────────────────────────────┐        │    │
│  │  │  Pod (running security tool)          │        │    │
│  │  │  - Main container (nmap, nikto, etc)  │        │    │
│  │  │  - Fluent Bit sidecar (log shipping)  │        │    │
│  │  └───────────────────────────────────────┘        │    │
│  │                                                    │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  External Services (Optional)                      │    │
│  │  - Loki (log aggregation)                          │    │
│  │  - Prometheus (metrics)                            │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Manager (Controller Runtime)

The Manager is the main entry point of KTtack, implemented in `cmd/main.go`. It:

- Initializes the Kubernetes Scheme and client
- Sets up webhook servers for CRD validation
- Registers all reconcilers (SecurityAttack, Enumeration, Liveness)
- Manages metrics and health endpoints
- Implements TLS certificate handling for webhooks

**Key Responsibilities:**
- Bootstrap controller-runtime
- Configure RBAC and authentication
- Setup metrics server (default port: 8080)
- Setup webhook server (default port: 9443)

### 2. Reconcilers

Each reconciler implements the core business logic for a specific CRD type.

#### SecurityAttackReconciler (`internal/controller/securityattack_controller.go`)

Handles `SecurityAttack` resources and orchestrates security testing operations.

**Responsibilities:**
- Watch SecurityAttack custom resources
- Create Job or CronJob based on `Periodic` flag
- Build pod specifications with proper tool configuration
- Update status with job state and execution time
- Handle resource cleanup

**Reconciliation Loop:**
1. Load tool specifications from ConfigMap
2. Fetch SecurityAttack resource
3. Determine if periodic (CronJob) or one-time (Job)
4. Create or update the corresponding Kubernetes job
5. Update status with job information

#### EnumerationReconciler

Similar to SecurityAttackReconciler, specialized for enumeration tasks.

#### LivenessReconciler

Handles liveness probes and health checks.

### 3. Tool Management Components

#### ToolSpecManager (`internal/controller/toolspec.go`)

Manages tool specifications and their configurations.

**Responsibilities:**
- Parse tool specifications from ConfigMap
- Validate tool definitions
- Provide tool metadata (image, default args, etc.)
- Support tool discovery and validation

#### ToolsConfigurator

Handles tool-specific configuration:

- Build pod specifications for tools
- Inject environment variables
- Configure proxy settings
- Map tool arguments to pod commands
- Configure debug/logging modes

### 4. Job Builders

Responsible for constructing Kubernetes Job and CronJob objects:

- **BuildJob**: Creates one-time Job objects
- **BuildCronJob**: Creates CronJob objects with schedule
- Handle pod template creation
- Attach Fluent Bit sidecar for logging

## Custom Resource Definitions (CRDs)

### SecurityAttack CRD

```yaml
apiVersion: kttack.io/v1alpha1
kind: SecurityAttack
metadata:
  name: example-attack
  namespace: security-testing
spec:
  # Type of attack: Enumeration, Vulnerability, Exploitation
  attackType: Enumeration
  
  # Target system
  target: "192.168.1.0/24"
  
  # Security tool to use
  tool: nmap
  
  # Optional: HTTP proxy
  http_proxy: "http://proxy:8080"
  
  # Optional: Additional environment variables
  additional_env:
    - name: CUSTOM_VAR
      value: "value"
  
  # Optional: Run on schedule
  periodic: false
  schedule: "0 2 * * *"  # 2 AM daily (if periodic: true)
  
  # Optional: Additional tool arguments
  args:
    - "-sV"
    - "-A"
  
  # Optional: Enable debug mode (output to stdout)
  debug: false

status:
  # Last execution timestamp
  lastExecuted: "2025-01-01T10:30:00Z"
  
  # Created Job/CronJob name
  jobName: "example-attack-12345"
  
  # Current state: Pending, Running, Completed, Failed
  state: Running
```

### Enumeration CRD

Similar structure with specific fields for enumeration operations.

### Liveness CRD

Defines health checks and availability probes.

## Controller Reconciliation Flow

The reconciliation flow follows a standard Kubernetes operator pattern:

```
Event Triggered
    ↓
┌─────────────────────┐
│ Reconcile Called    │
│ with Request        │
└─────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Load Tool Configurations                │
│ (from ConfigMap - kttack-tool-specs)           │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Fetch SecurityAttack Resource           │
│ (check if exists, handle deletion)      │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Determine Execution Type                │
│ Periodic=true → CronJob                 │
│ Periodic=false → Job                    │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Build Job/CronJob Specification         │
│ - Pod spec with security tool           │
│ - Fluent Bit sidecar (if debug=false)   │
│ - Resource limits                       │
│ - Environment variables                 │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Create or Update Job/CronJob            │
│ (detect if already exists)              │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Update SecurityAttack Status            │
│ - Set state (Running/Completed/Failed)  │
│ - Record lastExecuted timestamp         │
│ - Store jobName                         │
└─────────────────────────────────────────┘
    ↓
Return Result (Requeue if needed)
```

## Job and CronJob Management

### One-Time Job Execution

When `Periodic: false`, KTtack creates a Kubernetes `Job`:

1. **Job Name**: `{securityattack-name}`
2. **Restart Policy**: `Never` (fail on error)
3. **Backoff Limit**: 3 (retries)
4. **TTL**: 3600 seconds (cleanup after 1 hour)
5. **Pod Template**:
   - Main container: executes security tool
   - Sidecar: Fluent Bit (if debug=false)

### Periodic Execution (CronJob)

When `Periodic: true`, KTtack creates a Kubernetes `CronJob`:

1. **CronJob Name**: `{securityattack-name}-cronjob`
2. **Schedule**: Standard cron format (e.g., `0 2 * * *`)
3. **Concurrency Policy**: `Forbid` (prevent overlapping runs)
4. **History Limit**: Keep last 3 failed, 1 successful
5. **Pod Template**: Same as one-time job

### Pod Structure

```
Pod
├── Container: security-tool
│   ├── Image: defined in ToolSpec
│   ├── Args: tool arguments + target + proxy
│   ├── Env: tool-specific environment variables
│   ├── VolumeMounts: /logs (for log output)
│   └── Resources: limits (CPU, memory)
│
└── Container: fluent-bit (sidecar, if debug=false)
    ├── Image: fluent/fluent-bit
    ├── VolumeMounts: /logs (read), fluent-bit-config
    ├── Env: LOKI_* credentials
    └── Command: tail /logs/job.log → Loki
```

## Tool Integration

### Tool Specification Format

Tools are defined in the `kttack-tool-specs` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kttack-tool-specs
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
        image: "nikto:latest"
        default_args:
          - "-display 1234"
        timeout: 1800
        resource_limits:
          cpu: "250m"
          memory: "128Mi"
```

### Tool Execution

The flow for executing a tool:

1. **Tool Resolution**: Match requested tool with spec from ConfigMap
2. **Container Configuration**: Set image, args, environment
3. **Argument Building**: Combine default args with custom args and target
4. **Volume Mounting**: Attach shared volume for logs
5. **Pod Creation**: Submit configured pod to Kubernetes

## Logging and Monitoring

### Fluent Bit Sidecar Architecture

When `debug: false`:

1. **Main Container**: Redirects output to `/logs/job.log`
2. **Fluent Bit Sidecar**: 
   - Tails `/logs/job.log`
   - Applies filters (adds labels, enriches metadata)
   - Sends to Loki over HTTPS
3. **Log Labels**: job, namespace, tool, cluster

### Fluent Bit Configuration

```yaml
[INPUT]
  Name tail
  Path /logs/*.log
  Read_from_Head true

[FILTER]
  Name modify
  Add job=${JOB_NAME}
  Add namespace=${NAMESPACE}
  Add tool=${TOOL_TYPE}
  Add cluster=${CLUSTER_NAME}

[OUTPUT]
  Name loki
  host ${LOKI_HOST}
  port ${LOKI_PORT}
  http_user ${LOKI_USER}
  http_passwd ${LOKI_PASSWORD}
```

See [Fluent Bit Sidecar Documentation](./FLUENT_BIT_SIDECAR.md) for complete details.

### Metrics

KTtack exposes Prometheus metrics:

- `kttack_securityattack_total`: Total SecurityAttacks created
- `kttack_job_duration_seconds`: Job execution duration
- `kttack_tool_invocations`: Tool execution count by type

Metrics are exposed on `http://:8080/metrics`.

## Security and RBAC

### ServiceAccount and Roles

KTtack requires specific Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: manager-role
rules:
# SecurityAttack CRD
- apiGroups: ["kttack.io"]
  resources: ["securityattacks", "securityattacks/status"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Jobs and CronJobs
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Pods and ConfigMaps
- apiGroups: [""]
  resources: ["pods", "configmaps"]
  verbs: ["get", "list", "watch"]
```

### Pod Security

- **Non-root User**: Pods run as UID 65532
- **Read-only Filesystem**: Root filesystem is read-only
- **Network Policies**: Optional network isolation between testing pods

### Secret Management

- **Loki Credentials**: Stored in `kttack-system` namespace Secret
- **Tool Secrets**: Can be injected via environment variables
- **TLS Certificates**: Managed by cert-manager for webhook TLS

## Data Flow Example

### Complete flow for creating a SecurityAttack:

```
User
  ↓ kubectl apply -f securityattack.yaml
Kubernetes API Server
  ↓ (CRD stored in etcd)
SecurityAttackReconciler
  ↓ (watch event triggered)
Load kttack-tool-specs ConfigMap
  ↓ (resolve nmap configuration)
Validate target and tool
  ↓
Build Job Spec
  ├─ Container 1: nmap with args
  └─ Container 2: fluent-bit sidecar
  ↓ (create in Kubernetes)
Kubernetes Batch Controller
  ↓ (creates Pod)
Pod Scheduler
  ↓ (assigns node)
Kubelet
  ├─ Pulls images
  ├─ Creates containers
  ├─ nmap starts scanning
  └─ Fluent Bit tails logs
  ↓
Logs written to /logs/job.log
  ↓
Fluent Bit reads and forwards
  ↓
Loki (log aggregation)
  ↓ (accessible via Grafana)
```

## Extensibility

### Adding New CRD Types

To add a new CRD (e.g., `VulnerabilityAssessment`):

1. **Define API Types** in `api/v1alpha1/vulnerabilityassessment_types.go`
2. **Create Reconciler** in `internal/controller/vulnerabilityassessment_controller.go`
3. **Register with Manager** in `cmd/main.go`
4. **Generate CRD** with `make manifests`
5. **Add RBAC** rules to reconciler

For more info on this topic, refer to this [example](./NEW_CRD_EXAMPLE.md).

### Adding New Tool Support

To support a new security tool:

1. **Create Tool Image**: Build or use existing Docker image
2. **Define ToolSpec**: Add entry to `kttack-tool-specs` ConfigMap
3. **Test**: Create SecurityAttack with new tool
4. **Document**: Update tool configuration examples

## Performance Considerations

- **Controller Concurrency**: Configurable worker threads
- **Job Timeouts**: Configurable per tool in ToolSpec
- **Pod Resource Limits**: Prevent runaway resource consumption
- **Log Volume**: Fluent Bit tail plugin with rotation
- **Namespace Scoping**: RBAC limits controller to designated namespaces

## Troubleshooting Guide

### Common Issues

**Issue**: SecurityAttack stuck in "Pending"

- Check if kttack-tool-specs ConfigMap exists
- Verify tool image is accessible
- Check node capacity (CPU/memory limits)

**Issue**: Logs not appearing in Loki

- Verify Fluent Bit sidecar started: `kubectl logs <pod> -c fluent-bit`
- Check Loki credentials in Secret
- Verify network connectivity to Loki endpoint

**Issue**: CronJob not executing

- Verify cron schedule syntax
- Check CronJob status: `kubectl describe cronjob <name>`
- Review controller logs for scheduling errors

## References

- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Kubebuilder Documentation](https://book.kubebuilder.io/)
- [Controller Runtime Documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Fluent Bit Documentation](https://docs.fluentbit.io/)
