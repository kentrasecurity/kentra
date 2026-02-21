# Logging with Fluent Bit and Loki

Kentra supports centralized logging with [FluentBit](https://fluentbit.io/) and [Loki](https://grafana.com/oss/loki/).

With a standard Kentra installation, logs are written directly to the container's default output (stdout/stderr).

To centralize logs from all commands, we use FluentBit + Loki. This setup allows us to aggregate all logs into a single location, making it much easier to monitor and visualize them through the dashboard.

## Fluent Bit Sidecar for Logging in Loki

When a **CustomResource** is created:

1. **If `debug: false` (default)**:
   - The enumeration job redirects output to `/logs/job.log`
   - A Fluent Bit sidecar monitors the file `/logs/job.log`
   - Fluent Bit sends logs to Loki with the following labels:
     - `job`: name of the Enumeration
     - `namespace`: namespace where the pod is running
     - `tool`: type of tool used (nmap, nikto, etc.)
     - `cluster`: name of the cluster (to be configured in Secret)

2. **If `debug: true`**:
   - The enumeration job writes directly to stdout
   - No sidecar is added
   - Logs are available via `kubectl logs`

### Required Configuration

#### Secret: `loki-credentials` in `kentra-system`

Contains credentials and configurations for Loki:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: loki-credentials
  namespace: kentra-system
type: Opaque
stringData:
  loki-host: "loki.k3s.chungo.home"
  loki-port: "443"
  loki-tls: "true"
  loki-tls-verify: "false"
  loki-tenant-id: "1"
  loki-user: "root-user"
  loki-password: "supersecretpassword"
  cluster-name: "k3s"
```

#### ConfigMap: `fluent-bit-config` in `kentra-system`

Contains the Fluent Bit configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
  namespace: kentra-system
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
        Tag               kentra.job.*

    [FILTER]
        Name    modify
        Match   *
        Add     cluster ${CLUSTER_NAME}
        Add     component job
        Add     app kentra

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
        labels job=${JOB_NAME},namespace=${NAMESPACE},tool=${TOOL_TYPE},cluster=${CLUSTER_NAME}
        label_keys job,namespace,tool,cluster
```

### Deployment

Apply the configuration files:

```bash
kubectl apply -f config/default/loki-secret.yaml
kubectl apply -f config/default/fluent-bit-config.yaml
```

### Usage Example

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Enumeration
metadata:
  name: nmap-example
  namespace: default
spec:
  target: "192.168.1.0/24"
  tool: nmap
  debug: false  # Enables the Fluent Bit sidecar
  periodic: false
```

### Querying Logs in Loki

After execution, you can search logs with queries like:

```logql
{job="nmap-example", tool="nmap", namespace="default"}
```

Or filter for errors:

```logql
{cluster="k3s", app="kentra"} |= "error"
```

### Troubleshooting

#### Logs are not being sent to Loki

1. Verify that the Secret `loki-credentials` exists and has the correct values:
   ```bash
   kubectl describe secret loki-credentials -n kentra-system
   ```

2. Verify that the ConfigMap `fluent-bit-config` exists:
   ```bash
   kubectl describe configmap fluent-bit-config -n kentra-system
   ```

3. Check the logs of the Fluent Bit sidecar:
   ```bash
   kubectl logs <pod> -c fluent-bit-sidecar
   ```

#### Connection to Loki refused

- Verify that `loki-host` is reachable from the cluster
- Verify that `loki-port` is correct
- If `loki-tls` is `true`, make sure the certificates are valid
- If you use a self-signed certificate, leave `loki-tls-verify` as `false`

#### File `/logs` not found

- Make sure that `debug: false` in your Enumeration
- Verify that the enumeration job is creating the file `/logs/job.log`

### Supported Environment Variables

The Fluent Bit sidecar receives the following environment variables:

- `LOKI_HOST`: Loki server host
- `LOKI_PORT`: Loki server port
- `LOKI_TLS`: Whether to use TLS ("true" or "false")
- `LOKI_TLS_VERIFY`: Verify TLS certificate ("true" or "false")
- `LOKI_TENANT_ID`: Tenant ID in Loki
- `LOKI_USER`: Username for Loki
- `LOKI_PASSWORD`: Password for Loki
- `CLUSTER_NAME`: Name of the cluster (from Secret)
- `NAMESPACE`: Namespace of the pod
- `JOB_NAME`: Name of the Enumeration
- `TOOL_TYPE`: Type of tool used

### Installation via Helm

Use the following values in the Helm installation:

```yaml
loki:
  enabled: true
  minio:
    enabled: true
    persistence:
      size: 10Gi

  singleBinary:
    persistence:
      enabled: true
      size: 5Gi
```
