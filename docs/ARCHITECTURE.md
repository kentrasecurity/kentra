# Kentra Architecture

This document provides a comprehensive overview of the Kentra Kubernetes Operator architecture, including its components, design patterns, and interactions.

## Table of Contents

- [Kentra Architecture](#kentra-architecture)
  - [Table of Contents](#table-of-contents)
  - [High-Level Architecture](#high-level-architecture)
  - [List of CRDs](#list-of-crds)
    - [Attacks](#attacks)
    - [Pools](#pools)
  - [Flow Example with nmap scan](#flow-example-with-nmap-scan)

## High-Level Architecture

<img src="./img/logical_arch.svg" alt="logical_arch" width="400" />

The workflow begins when an attack is defined in a YAML file and applied to the cluster, either manually or via a git push. This action creates a Custom Resource (CR) which the Kentra Controller monitors.

The entrypoint in `cmd/main.go` initializes the controller manager, registers specific controllers for each Custom Resource Definition (CRD), and sets up essential health checks. Supporting this, `cmd/manager.go` bootstraps the controller-runtime, configures RBAC for secure access, and manages the metrics and webhook servers.

Once a CR is detected, the reconciler translates the high-level specification into a standard Kubernetes Job. This Job launches a Pod containing two specific components: a main container that executes the security tool's CLI command and an optional Fluent Bit sidecar. If Loki is configured, the sidecar captures the tool's output and forwards it to Loki, allowing users to monitor live attack progress and results through Grafana. If not, the logs are shown in the pod logs.

## List of CRDs

### Attacks 
  | CRD | Description |
 | :--- | :--- |
| Liveness | Availability and health verification. |
 | Exploit | Active vulnerability testing. |
 | Osint | Reconnaissance and data gathering. |

### Pools
 | CRD | Description |
 | :--- | :--- |
 | Storage | Persistent data and artifact repositories. |
 | Asset | Grouped entities for miscellaneous purpose. |
 | Target | Grouped objects for scanning. |

## Flow Example with nmap scan

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant KubernetesAPI as Kubernetes API Server
    participant Controller as Attack Reconciler
    participant Kubelet as Worker Node (Kubelet)
    participant Loki as Loki/Grafana

    User->>KubernetesAPI: kubectl apply -f attack.yaml
    KubernetesAPI->>Controller: New Attack detected
    
    Note over Controller: 1. Load kentra-tool-specs<br/>2. Resolve tool args<br/>3. Build Job Spec
    
    Controller->>KubernetesAPI: Create Kubernetes Job
    
    Note over KubernetesAPI: Job Controller<br/>creates Pod & Scheduler assigns Node
    
    KubernetesAPI->>Kubelet: Pull images & Start Pod
    
    rect rgb(240, 240, 240)
        Note over Kubelet: tool runs & writes to /logs/job.log
        Note over Kubelet: Fluent Bit sidecar reads /logs/job.log
    end

    Kubelet->>Loki: Push logs via Fluent Bit
    Loki-->>User: View scan results
```

