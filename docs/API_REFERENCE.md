# Kentra API Reference

Complete API reference for Kentra Custom Resource Definitions (CRDs).

## Table of Contents

1. [SecurityAttack API](#securityattack-api)
2. [Enumeration API](#enumeration-api)
3. [Liveness API](#liveness-api)
4. [Common Fields](#common-fields)
5. [Status Conditions](#status-conditions)

## SecurityAttack API

The `SecurityAttack` resource defines a security testing operation.

### API Version and Kind

```yaml
apiVersion: kentra.sh/v1alpha1
kind: SecurityAttack
```

### SecurityAttackSpec

```yaml
spec:
  # (Required) Type of security attack
  # Valid values: Enumeration, Vulnerability, Exploitation
  attackType: string
  
  # (Required) Target system (IP address, hostname, or CIDR)
  target: string
  
  # (Required) Security tool to execute
  tool: string
  
  # (Optional) HTTP proxy URL
  # Format: http://proxy-host:port
  http_proxy: string
  
  # (Optional) Additional environment variables
  additional_env:
    - name: VARIABLE_NAME
      value: "variable_value"
    - name: ANOTHER_VAR
      value: "value"
  
  # (Optional) Enable periodic execution
  # Default: false
  periodic: boolean
  
  # (Optional) Cron schedule (required if periodic is true)
  # Format: minute hour day month weekday
  # Examples:
  #   "0 2 * * *" = 2 AM every day
  #   "0 3 * * 0" = 3 AM on Sundays
  #   "0 */6 * * *" = Every 6 hours
  schedule: string
  
  # (Optional) Additional tool arguments
  args:
    - "-sV"
    - "-A"
    - "--custom-arg"
  
  # (Optional) Enable debug mode
  # When true: output goes to stdout/pod logs
  # When false: output redirected to /logs/job.log with Fluent Bit sidecar
  # Default: false
  debug: boolean
  
  # (Optional) Job timeout in seconds
  # If not set, uses tool-spec default timeout
  timeout: integer
  
  # (Optional) Retry policy
  backoffLimit: integer
  
  # (Optional) Time-To-Live for completed job (in seconds)
  ttlSecondsAfterFinished: integer
```

### SecurityAttackStatus

```yaml
status:
  # Timestamp of last execution (RFC3339 format)
  lastExecuted: string
  
  # Name of created Job or CronJob
  jobName: string
  
  # Current state of the attack
  # Valid values: Pending, Running, Completed, Failed
  state: string
  
  # Human-readable message providing additional details
  message: string
```

### Complete SecurityAttack Example

```yaml
apiVersion: kentra.sh/v1alpha1
kind: SecurityAttack
metadata:
  name: network-enumeration
  namespace: security-testing
  labels:
    team: security
    environment: production
  annotations:
    description: "Daily network enumeration scan"
spec:
  attackType: Enumeration
  target: "192.168.0.0/16"
  tool: nmap
  
  # One-time execution
  periodic: false
  debug: false
  
  # Nmap arguments
  args:
    - "-sV"
    - "-A"
    - "-Pn"
    - "-T4"
  
  # Cleanup after 3600 seconds
  ttlSecondsAfterFinished: 3600
  
  # Retry up to 3 times
  backoffLimit: 3

status:
  lastExecuted: "2025-01-10T10:30:00Z"
  jobName: "network-enumeration-abc123"
  state: Completed
```

## Enumeration API

The `Enumeration` resource is specialized for network and service enumeration tasks.

### API Version and Kind

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Enumeration
```

### EnumerationSpec

Extends SecurityAttackSpec with enumeration-specific fields:

```yaml
spec:
  # (Required) Enumeration target type
  # Valid values: Network, WebService, DNS, Service
  targetType: string
  
  # (Inherited from SecurityAttack)
  target: string
  tool: string
  
  # (Optional) Enumerate specific ports
  ports:
    - 80
    - 443
    - 8080
  
  # (Optional) Port range
  portRange: "1-65535"
  
  # (Optional) Use OS detection
  osDetection: boolean
  
  # (Optional) Use version detection
  versionDetection: boolean
  
  # (Optional) Intensive scanning
  aggressiveScan: boolean
  
  # (Inherited from SecurityAttack)
  periodic: boolean
  schedule: string
  debug: boolean
```

### Enumeration Example

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Enumeration
metadata:
  name: dns-enumeration
  namespace: security-testing
spec:
  targetType: DNS
  target: "example.com"
  tool: dig
  
  periodic: true
  schedule: "0 1 * * *"  # Daily at 1 AM
  
  debug: false
  
  args:
    - "+short"
    - "@8.8.8.8"
```

## Liveness API

The `Liveness` resource defines health checks and availability probes.

### API Version and Kind

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Liveness
```

### LivenessSpec

```yaml
spec:
  # (Required) Probe type
  # Valid values: HTTP, TCP, Command
  probeType: string
  
  # (Required) Target to probe
  target: string
  
  # (Optional) HTTP endpoint (for HTTP probe type)
  httpPath: string
  
  # (Optional) Expected HTTP status code
  httpStatus: integer
  
  # (Optional) TCP port (for TCP probe type)
  tcpPort: integer
  
  # (Optional) Command to execute (for Command probe type)
  command:
    - "/bin/sh"
    - "-c"
    - "curl -f http://target/health"
  
  # (Optional) Probe interval (seconds)
  interval: integer
  
  # (Optional) Probe timeout (seconds)
  timeout: integer
  
  # (Optional) Initial delay before first probe (seconds)
  initialDelaySeconds: integer
  
  # (Optional) Number of successive successes to be considered healthy
  successThreshold: integer
  
  # (Optional) Number of failures to be considered unhealthy
  failureThreshold: integer
  
  # (Optional) Enable periodic probing
  periodic: boolean
  schedule: string
```

### Liveness Example

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Liveness
metadata:
  name: website-health
  namespace: security-testing
spec:
  probeType: HTTP
  target: "https://example.com"
  httpPath: "/health"
  httpStatus: 200
  
  interval: 60
  timeout: 10
  initialDelaySeconds: 30
  
  successThreshold: 1
  failureThreshold: 3
  
  periodic: true
  schedule: "*/5 * * * *"  # Every 5 minutes
```

## Common Fields

All Kentra resources share common Kubernetes metadata fields:

### Metadata

```yaml
metadata:
  # (Required) Resource name (DNS-1123 subdomain)
  name: string
  
  # (Required) Namespace
  namespace: string
  
  # (Optional) Labels for organization and selection
  labels:
    app: kentra
    team: security
    environment: production
  
  # (Optional) Annotations for documentation
  annotations:
    description: "Description of what this resource does"
    owner: "team-name"
    jira-ticket: "SEC-123"
```

### Common Spec Fields

All security resources support these common fields:

```yaml
spec:
  # (Optional) HTTP proxy URL
  http_proxy: "http://proxy.example.com:8080"
  
  # (Optional) Additional environment variables
  additional_env:
    - name: VAR1
      value: "value1"
    - name: VAR2
      value: "value2"
  
  # (Optional) Run on schedule
  periodic: false
  schedule: "0 2 * * *"  # Required if periodic: true
  
  # (Optional) Debug mode
  debug: false
```

## Status Conditions

All Kentra resources report status conditions.

### State Values

| State | Description |
|-------|-------------|
| `Pending` | Resource created but not yet started |
| `Running` | Job is currently executing |
| `Completed` | Job finished successfully |
| `Failed` | Job failed with error |

### Status Example

```yaml
status:
  # Timestamp of last execution
  lastExecuted: "2025-01-10T10:30:00Z"
  
  # Current execution state
  state: Running
  
  # Human-readable message
  message: "Scanning network... (2% complete)"
  
  # For periodic executions
  nextScheduledTime: "2025-01-11T02:00:00Z"
  
  # For failed executions
  lastError: "Connection timeout to target"
```

## Field Validation

Kentra performs validation on all resource fields:

### Type Validation

- `attackType`: Must be one of `Enumeration`, `Vulnerability`, `Exploitation`
- `target`: Valid IP, hostname, or CIDR notation
- `tool`: Must exist in kentra-tool-specs ConfigMap
- `state`: Must be `Pending`, `Running`, `Completed`, or `Failed`

### Format Validation

- `schedule`: Must be valid cron expression (5 or 6 fields)
- `http_proxy`: Must be valid URL format
- `timeout`: Must be positive integer
- `ttlSecondsAfterFinished`: Must be positive integer or zero

### Business Logic Validation

- If `periodic: true`, `schedule` must be provided
- Tool must be defined in `kentra-tool-specs` ConfigMap
- Target must be reachable or valid hostname
- Resource name must follow DNS-1123 subdomain rules

## API Versioning

Kentra uses `v1alpha1` API version. This is an experimental version and may change in future releases.

### Future Compatibility

- Upgrade path to `v1beta1` will be provided before `v1` release
- Deprecated fields will trigger warnings before removal
- Migration guides will be provided for major version changes

## Tool Specifications

Tools are referenced by name in SecurityAttack resources. Available tools are defined in the `kentra-tool-specs` ConfigMap.

### Tool Definition Structure

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kentra-tool-specs
  namespace: kentra-system
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
```

### Adding Custom Tools

To add a new tool, update the `kentra-tool-specs` ConfigMap:

```bash
kubectl patch configmap kentra-tool-specs -n kentra-system \
  --type merge -p '{"data":{"tools.yaml":"..."}}'
```

## Practical Examples

### Example 1: Web Application Vulnerability Scan

```yaml
apiVersion: kentra.sh/v1alpha1
kind: SecurityAttack
metadata:
  name: web-app-vuln-scan
  namespace: security-testing
spec:
  attackType: Vulnerability
  target: "https://myapp.example.com"
  tool: nikto
  periodic: false
  debug: false
  args:
    - "-h"
    - "myapp.example.com"
    - "-Format"
    - "json"
```

### Example 2: Daily Network Scan

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Enumeration
metadata:
  name: daily-network-scan
  namespace: security-testing
spec:
  targetType: Network
  target: "10.0.0.0/8"
  tool: nmap
  periodic: true
  schedule: "0 2 * * *"
  debug: false
  osDetection: true
  versionDetection: true
  aggressiveScan: false
```

### Example 3: Health Check Probe

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Liveness
metadata:
  name: api-health-check
  namespace: security-testing
spec:
  probeType: HTTP
  target: "https://api.example.com"
  httpPath: "/health"
  httpStatus: 200
  interval: 60
  timeout: 10
  periodic: true
  schedule: "*/5 * * * *"
```

## Related Resources

- [Architecture Guide](./ARCHITECTURE.md) - Detailed system architecture
- [Installation Guide](./INSTALLATION_GUIDE.md) - Installation and setup
- [Getting Started](./GETTING_STARTED.md) - Quick start examples
- [Fluent Bit Documentation](./FLUENT_BIT_SIDECAR.md) - Logging configuration
