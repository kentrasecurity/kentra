# Getting Started with Kentra

This guide will help you create your first security test with Kentra in just a few minutes.

## Prerequisites

- Kentra installed and running (see [Installation Guide](./INSTALLATION_GUIDE.md))
- `kubectl` configured to access your cluster
- A Kubernetes namespace for security testing (e.g., `security-testing`)

## Step 1: Create a Namespace

```bash
kubectl create namespace security-testing
```

## Step 2: Create Your First SecurityAttack

Create a file named `my-first-attack.yaml`:

```yaml
apiVersion: kentra.sh/v1alpha1
kind: SecurityAttack
metadata:
  name: first-nmap-scan
  namespace: security-testing
spec:
  # Type of attack
  attackType: Enumeration
  
  # Target to scan (IP, hostname, or CIDR)
  target: "192.168.1.0/24"
  
  # Security tool to use
  tool: nmap
  
  # One-time execution (not periodic)
  periodic: false
  
  # Additional arguments for the tool
  args:
    - "-sV"
    - "-A"
  
  # Enable debug mode to see output in pod logs
  debug: true
```

## Step 3: Apply the SecurityAttack

```bash
kubectl apply -f my-first-attack.yaml
```

## Step 4: Monitor Execution

Watch the attack progress:

```bash
# List all security attacks
kubectl get securityattacks -n security-testing

# Watch the specific attack
kubectl describe securityattack first-nmap-scan -n security-testing

# View the job status
kubectl get jobs -n security-testing

# View pod logs (real-time)
kubectl logs -n security-testing job/first-nmap-scan -f
```

## Step 5: Review Results

Once the job completes:

```bash
# Get the pod name
POD=$(kubectl get pods -n security-testing -l job-name=first-nmap-scan -o jsonpath='{.items[0].metadata.name}')

# View full output
kubectl logs -n security-testing $POD

# Or save output to file
kubectl logs -n security-testing $POD > scan-results.txt
```

## Example: Periodic Scanning

To run the same scan automatically on a schedule, create `periodic-nmap.yaml`:

```yaml
apiVersion: kentra.sh/v1alpha1
kind: SecurityAttack
metadata:
  name: scheduled-nmap-scan
  namespace: security-testing
spec:
  attackType: Enumeration
  target: "192.168.1.0/24"
  tool: nmap
  
  # Enable periodic execution
  periodic: true
  
  # Run every day at 2 AM
  schedule: "0 2 * * *"
  
  args:
    - "-sV"
    - "-A"
  
  debug: false
```

Apply it:

```bash
kubectl apply -f periodic-nmap.yaml

# Check the CronJob
kubectl get cronjobs -n security-testing

# View CronJob details
kubectl describe cronjob scheduled-nmap-scan-cronjob -n security-testing
```

## Example: Using HTTP Proxy

If your security tools need to connect through a proxy:

```yaml
apiVersion: kentra.sh/v1alpha1
kind: SecurityAttack
metadata:
  name: nmap-behind-proxy
  namespace: security-testing
spec:
  attackType: Enumeration
  target: "10.0.0.0/8"
  tool: nmap
  
  # Specify HTTP proxy
  http_proxy: "http://proxy.example.com:8080"
  
  periodic: false
  debug: true
  
  args:
    - "-sV"
```

## Example: Custom Environment Variables

Pass custom environment variables to your tools:

```yaml
apiVersion: kentra.sh/v1alpha1
kind: SecurityAttack
metadata:
  name: custom-env-scan
  namespace: security-testing
spec:
  attackType: Enumeration
  target: "example.com"
  tool: nikto
  
  # Custom environment variables
  additional_env:
    - name: CUSTOM_WORDLIST
      value: "/wordlists/custom.txt"
    - name: TIMEOUT
      value: "30"
  
  periodic: false
  debug: true
  
  args:
    - "-h"
    - "example.com"
```

## Viewing Centralized Logs (Optional)

If you've configured Fluent Bit and Loki:

### View logs in Loki

```bash
# Port forward to Loki (if not exposed)
kubectl port-forward -n observability svc/loki 3100:3100

# Query logs via Loki API
curl 'http://localhost:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={job="first-nmap-scan"}'
```

### View logs in Grafana

1. Add Loki as a data source in Grafana
2. Create a dashboard with LogQL queries:
   ```
   {job="first-nmap-scan"}
   ```

## Troubleshooting

### Pod stuck in Pending

```bash
# Check pod events
kubectl describe pod -n security-testing <pod-name>

# Common issues:
# - Image not found: kubectl get pod -o yaml to check image
# - Insufficient resources: kubectl top nodes
# - Node selector mismatch: kubectl get nodes --show-labels
```

### Job Failed

```bash
# Check job status
kubectl describe job -n security-testing <job-name>

# View pod logs for error
kubectl logs -n security-testing <pod-name>

# Common issues:
# - Tool not found in registry
# - Target unreachable
# - Insufficient permissions
```

### Tool Not Found

```bash
# Verify tool is defined in kentra-tool-specs ConfigMap
kubectl get cm -n kentra-system kentra-tool-specs -o yaml

# Check available tools
kubectl describe cm -n kentra-system kentra-tool-specs
```

### CronJob Not Executing

```bash
# Check CronJob status
kubectl describe cronjob -n security-testing <name>

# View recent job history
kubectl get jobs -n security-testing -l cronjob-name=<cronjob-name>

# Check cron syntax (must be valid cron format)
# Format: minute hour day month weekday
# Example: 0 2 * * * = 2 AM every day
```

## Next Steps

1. **Explore Available Tools**: Check `config/default/kentra-tool-specs.yaml` for available security tools
2. **Set Up Logging**: Configure Fluent Bit for centralized log aggregation (see [Fluent Bit Documentation](./FLUENT_BIT_SIDECAR.md))
3. **Create Custom Tools**: Add new security tools to kentra-tool-specs ConfigMap
4. **Scale Testing**: Create multiple attacks targeting different systems
5. **Integrate Monitoring**: Set up Prometheus metrics and alerting

## Common Use Cases

### Daily Network Scanning

```yaml
# Runs Nmap every day at 2 AM
periodic: true
schedule: "0 2 * * *"
```

### Weekly Vulnerability Assessment

```yaml
# Runs on Sundays at 3 AM
periodic: true
schedule: "0 3 * * 0"
```

### Monthly Full Penetration Test

```yaml
# Runs on the 1st of each month at 1 AM
periodic: true
schedule: "0 1 1 * *"
```

### Continuous Web App Testing

```yaml
# Runs every 6 hours
periodic: true
schedule: "0 */6 * * *"
```

## Advanced Configuration

### Resource Limits

Control pod resource consumption by adding to your tool spec:

```yaml
tool: custom-scanner
# In kentra-tool-specs ConfigMap:
resource_limits:
  cpu: "1000m"
  memory: "512Mi"
```

### Tool-Specific Arguments

Each tool supports different arguments. Check the tool's documentation:

```bash
# For Nmap
args:
  - "-sV"  # Version detection
  - "-A"   # Aggressive scan
  - "-O"   # OS detection

# For Nikto
args:
  - "-Display"
  - "1234"  # Output format

# For Feroxbuster
args:
  - "-s"
  - "200,204,301,302"  # Status codes to include
```

## Support

For detailed information:

- **Architecture**: See [Architecture Guide](./ARCHITECTURE.md)
- **Installation**: See [Installation Guide](./INSTALLATION_GUIDE.md)
- **Logging**: See [Fluent Bit Documentation](./FLUENT_BIT_SIDECAR.md)
- **Repository**: [GitHub - Kentra](https://github.com/kentrasecurity/kentra)

## Tips & Best Practices

1. **Always Start with Debug Mode**: Set `debug: true` to verify your scans work correctly
2. **Test with Small Targets First**: Start with single IPs before scanning large CIDR blocks
3. **Use Meaningful Names**: Give your SecurityAttacks descriptive names
4. **Monitor Resource Usage**: Use `kubectl top pods` to monitor resource consumption
5. **Implement RBAC**: Restrict SecurityAttack creation to authorized teams
6. **Log Everything**: Enable centralized logging for audit trails
7. **Schedule Off-Peak**: Run large scans during low-traffic periods
8. **Version Your Configs**: Store SecurityAttack manifests in version control

Happy scanning! 🔒
