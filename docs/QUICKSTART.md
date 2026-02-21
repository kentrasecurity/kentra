# Run Your First Attack

Before running your first attack, ensure that the tools you want to use are properly defined in [tool-specs.yaml](config/default/tool-specs.yaml). The following example shows a minimal tool-specs configuration with two commonly used tools: nmap and netcat.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kentra-tool-specs
  namespace: kentra-system
data:
  tools: |
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
    netcat:
      type: "enumeration"
      image: "subfuzion/netcat"
      commandTemplate: "nc {{.Args}} {{.Target.endpoint}} {{.Target.port}}"
      capabilities:
        add:
          - NET_RAW
```

Apply this configuration with:

```bash
kubectl apply -f tool-specs.yaml
```

## Render Engine

The `tool-specs` ConfigMap uses a template rendering system to dynamically construct commands:
- `{{.Args}}` is replaced with values from the **Attack Custom Resource** (e.g., Enumeration spec)
- `{{.Target.endpoint}}` and `{{.Target.port}}` are replaced with values from the **Target Custom Resource** (e.g., TargetPool spec)

This allows for flexible command construction based on the target specifications.


## Running Enumerations
Here's an example manifest with a TargetPool and two different Enumeration jobs:

```yaml
apiVersion: kentra.sh/v1alpha1
kind: TargetPool
metadata:
  name: targetpool-test
  namespace: kentra-system
spec:
  description: "Direct target for nmap"
  targets:  
    - name: test1
      endpoint: 
        - "192.168.1.0/32"
        - "192.168.2.0"
      port:
        - "22"
        - "80"
    - name: test2
      endpoint: 
        - "192.168.3.1"
      port:
        - "22-24"
---
apiVersion: kentra.sh/v1alpha1
kind: Enumeration
metadata:
  name: netcat-banner-grab
  namespace: kentra-system
spec:
  args:
    - '-v'
    - '-w'
    - '5'
  category: Reconnaissance
  debug: true
  periodic: false
  targetPool: targetpool-test
  tool: netcat
---
apiVersion: kentra.sh/v1alpha1
kind: Enumeration
metadata:
  name: nmap-scan
  namespace: kentra-system
spec:
  tool: "nmap"
  targetPool: targetpool-test
  category: "network-scan"
  args:
    - "-sV"
  debug: true
  periodic: false
```

This manifest executes two enumerations against multiple targets. The behavior depends on the tool's capabilities:

**For tools that support multiple inline hosts** (like [nmap](https://nmap.org/book/port-scanning-options.html)):
- The render engine creates a single pod with a single command:
```bash
nmap 192.168.1.0 192.168.1.1 192.168.1.2 ... -p 22
```

**For tools that don't easily support multiple inline targets** (like [netcat](https://linux.die.net/man/1/nc)):
- Kentra automatically creates multiple pod instances, one for each target:
```bash
nc 192.168.1.0 22
nc 192.168.1.1 22
nc 192.168.1.2 22
...
```

To execute this enumeration, create a `enumerations.yaml` manifest file and apply it

```bash
kubectl apply -f enumerations.yaml
```
The result of the will be the following

```
> kubectl get po,jobs,storagepool -n kentra-system
NAME                                            READY   STATUS      RESTARTS       AGE
pod/netcat-banner-grab-0-665x8                  1/1     Completed       0          47s
pod/netcat-banner-grab-0-dmz84                  1/1     Completed       0          19s
pod/netcat-banner-grab-0-kz96f                  1/1     Completed       0          69s
pod/netcat-banner-grab-1-6wg2d                  1/1     Completed       0          19s
pod/netcat-banner-grab-1-9zqcf                  1/1     Completed       0          69s
pod/netcat-banner-grab-1-skfvn                  1/1     Completed       0          48s
pod/netcat-banner-grab-2-g7wc4                  1/1     Completed       0          69s
pod/netcat-banner-grab-2-qk66s                  1/1     Completed       0          44s
pod/netcat-banner-grab-2-svdpr                  1/1     Completed       0          10s
pod/netcat-banner-grab-3-2w5c9                  1/1     Completed       0          43s
pod/netcat-banner-grab-3-jlj9l                  1/1     Completed       0          69s
pod/netcat-banner-grab-3-mjp94                  1/1     Completed       0          10s
pod/netcat-banner-grab-4-2c6mr                  1/1     Completed       0          44s
pod/netcat-banner-grab-4-skcg7                  1/1     Completed       0          69s
pod/netcat-banner-grab-4-tjr7f                  1/1     Completed       0          15s
pod/netcat-banner-grab-5-bc4dd                  1/1     Completed       0          44s
pod/netcat-banner-grab-5-nklqq                  1/1     Completed       0          69s
pod/netcat-banner-grab-5-pwfwv                  1/1     Completed       0          15s
pod/netcat-banner-grab-6-7z4rw                  1/1     Completed       0          69s
pod/netcat-banner-grab-6-tk6dl                  1/1     Completed       0          15s
pod/netcat-banner-grab-6-ws5n7                  1/1     Completed       0          44s
pod/nmap-scan-test1-r4ns7                       1/1     Completed       0          69s
pod/nmap-scan-test2-fkc49                       1/1     Completed       0          69s

NAME                             COMPLETIONS   DURATION   AGE
job.batch/netcat-banner-grab-0   1/1           69s        69s
job.batch/netcat-banner-grab-1   1/1           69s        69s
job.batch/netcat-banner-grab-2   1/1           69s        69s
job.batch/netcat-banner-grab-3   1/1           69s        69s
job.batch/netcat-banner-grab-4   1/1           69s        69s
job.batch/netcat-banner-grab-5   1/1           69s        69s
job.batch/netcat-banner-grab-6   1/1           69s        69s
job.batch/nmap-scan-test1        1/1           23s        69s
job.batch/nmap-scan-test2        1/1           24s        69s
```

## Scheduled Enumerations

For recurring scans, enable the **CronJob** functionality by setting:
- `periodic: true` - Enable scheduling
- `schedule: "* * * * *"` - Schedule in [standard Unix cron format](https://www.ibm.com/docs/en/db2/11.5.x?topic=task-unix-cron-format)

Example:
```yaml
spec:
  periodic: true
  schedule: "0 2 * * *"  # Run daily at 2 AM
  tool: nmap
  targetPool: targetpool-test
```

## Monitoring Results

After applying the manifest, you can monitor the created resources:

```bash
kubectl get enumeration -n kentra-system
kubectl get jobs -n kentra-system
kubectl get pods -n kentra-system
```

View logs from a specific scan:

```bash
kubectl logs <pod-name> -n kentra-system
```

For centralized logging, see the [Logging Documentation](./LOGGING.md) to set up Fluent Bit and Loki integration.