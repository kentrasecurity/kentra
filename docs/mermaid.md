# Kentra Architecture - Mermaid Diagram

## High-Level System Architecture

```mermaid
graph TB
    subgraph "Entry Point - cmd/"
        MAIN[main.go<br/>Entry Point]
        CONFIG[CLI flags & config parsing]
    end

    subgraph "API Layer - api/v1alpha1"
        subgraph "Attack CRDs"
            EXPLOIT[Exploit<br/>Exploitation Tools]
            ENUM[Enumeration<br/>Network Scanning]
            OSINT[Osint<br/>Intelligence Gathering]
            LIVENESS[Liveness<br/>Connectivity Checks]
        end

        subgraph "Pool CRDs"
            TARGETPOOL[TargetPool<br/>Centralized Targets]
            STORAGEPOOL[StoragePool<br/>S3 File References]
            ASSETPOOL[AssetPool<br/>Asset Groupings]
        end
    end

    subgraph "Controller Layer - internal/controller"
        subgraph "Attack Controllers"
            EXPLOIT_CTRL[ExploitReconciler]
            ENUM_CTRL[EnumerationReconciler]
            OSINT_CTRL[OsintReconciler]
            LIVENESS_CTRL[LivenessReconciler]
        end

        subgraph "Pool Controllers"
            TARGET_CTRL[TargetPoolReconciler]
            STORAGE_CTRL[StoragePoolReconciler]
            ASSET_CTRL[AssetPoolReconciler]
        end

        subgraph "Base Logic"
            BASE_ATTACK[BaseAttackReconciler<br/>internal/controller/base]
            BASE_POOL[BasePoolReconciler<br/>internal/controller/base]
        end
    end

    subgraph "Internal Modules - internal/controller"
        subgraph "Job Management"
            JOB_BUILDER[JobBuilder<br/>internal/controller/jobs]
            JOB_FACTORY[JobFactory Interface<br/>EnumerationJobFactory, etc.]
        end

        subgraph "Pod Construction"
            POD_BUILDER[PodBuilder<br/>internal/controller/pods]
        end

        subgraph "Configuration"
            TOOLS_CONFIG[ToolsConfigurator<br/>internal/controller/config]
            CONFIGMAP[ConfigMap Loader<br/>label: kentra.sh/resource-type]
        end

        subgraph "Resolvers"
            POOL_RESOLVER[PoolResolver<br/>internal/controller/resolvers]
        end

        subgraph "Services"
            SERVICES_PKG[ReverseShellHandler<br/>internal/controller/services]
        end

        subgraph "Utilities"
            UTILS[Utilities<br/>internal/controller/utils]
        end
    end

    subgraph "Kubernetes Resources"
        JOBS[Jobs/CronJobs]
        PODS[Pods]
        SVCS[Services]
        CONFIGMAPS[ConfigMaps]
    end

    MAIN --> CONFIG
    MAIN --> EXPLOIT_CTRL
    MAIN --> ENUM_CTRL
    MAIN --> OSINT_CTRL
    MAIN --> LIVENESS_CTRL
    MAIN --> TARGET_CTRL
    MAIN --> STORAGE_CTRL
    MAIN --> ASSET_CTRL

    EXPLOIT --> EXPLOIT_CTRL
    ENUM --> ENUM_CTRL
    OSINT --> OSINT_CTRL
    LIVENESS --> LIVENESS_CTRL

    TARGETPOOL --> TARGET_CTRL
    STORAGEPOOL --> STORAGE_CTRL
    ASSETPOOL --> ASSET_CTRL

    EXPLOIT_CTRL --> BASE_ATTACK
    ENUM_CTRL --> BASE_ATTACK
    OSINT_CTRL --> BASE_ATTACK
    LIVENESS_CTRL --> BASE_ATTACK

    TARGET_CTRL --> BASE_POOL
    STORAGE_CTRL --> BASE_POOL
    ASSET_CTRL --> BASE_POOL

    BASE_ATTACK --> JOB_FACTORY
    BASE_ATTACK --> TOOLS_CONFIG

    JOB_FACTORY --> JOB_BUILDER
    JOB_FACTORY --> POOL_RESOLVER

    JOB_BUILDER --> POD_BUILDER

    TOOLS_CONFIG --> CONFIGMAP
    CONFIGMAPS --> CONFIGMAP

    POOL_RESOLVER --> TARGETPOOL
    POOL_RESOLVER --> STORAGEPOOL
    POOL_RESOLVER --> ASSETPOOL

    EXPLOIT_CTRL --> SERVICES_PKG
    SERVICES_PKG --> SVCS

    JOB_BUILDER --> JOBS
    JOBS --> PODS

    POOL_RESOLVER --> UTILS
    JOB_BUILDER --> UTILS
```

## Detailed Reconciliation Flow

```mermaid
sequenceDiagram
    participant User
    participant K8s as Kubernetes API
    participant Reconciler as AttackReconciler
    participant Base as BaseAttackReconciler
    participant Factory as JobFactory
    participant Resolver as PoolResolver
    participant Config as ToolsConfigurator
    participant JobBuilder
    participant PodBuilder

    User->>K8s: Create/Update Attack Resource
    K8s->>Reconciler: Reconcile Event
    Reconciler->>Base: ReconcileAttack()

    Base->>Base: ValidateNamespace()
    Base->>Base: EnsureLabels()
    Base->>Config: LoadConfig()
    Config->>K8s: Get ConfigMaps
    Config-->>Base: Tool Specs

    Base->>Factory: ReconcileJobs()

    Factory->>Resolver: ResolveTargetPool()
    Resolver->>K8s: Get TargetPool
    Resolver->>Resolver: Expand CIDRs & Port Ranges
    Resolver-->>Factory: []ResolvedTarget

    Factory->>Resolver: ResolveStoragePool()
    Resolver->>K8s: Get StoragePool
    Resolver-->>Factory: File List

    Factory->>Resolver: ResolveAssetPool()
    Resolver->>K8s: Get AssetPool
    Resolver-->>Factory: Asset Items

    Factory->>Config: BuildCommand()
    Config-->>Factory: Templated Command

    Factory->>JobBuilder: ReconcileJob(AttackSpec)

    JobBuilder->>PodBuilder: BuildPodSpec()
    PodBuilder->>PodBuilder: Set tool image
    PodBuilder->>PodBuilder: Build command from template
    PodBuilder->>PodBuilder: Add volumes (input/output)
    PodBuilder->>PodBuilder: Add fluent-bit sidecar (if not debug)
    PodBuilder-->>JobBuilder: PodSpec

    JobBuilder->>JobBuilder: Build Job/CronJob
    JobBuilder->>K8s: Create/Update Job

    JobBuilder-->>Factory: Status Update Callback
    Factory-->>Reconciler: Update Status
    Reconciler->>K8s: Update Resource Status
```

## Attack Execution Flow

```mermaid
flowchart TD
    START([Attack Resource Created]) --> RECONCILE[Reconciler Triggered]

    RECONCILE --> VALIDATE{Validate<br/>Namespace?}
    VALIDATE -->|Invalid| ERROR1[Return Error]
    VALIDATE -->|Valid| LABELS[Ensure Labels<br/>kentra.sh/resource-type]

    LABELS --> LOADCONFIG[Load Tool Config<br/>from ConfigMaps]
    LOADCONFIG --> RESOLVE[Resolve Pool References]

    RESOLVE --> CHECKPOOL{Pool<br/>References?}
    CHECKPOOL -->|TargetPool| RESOLVETARGET[Resolve Target<br/>Expand CIDR & Ports]
    CHECKPOOL -->|StoragePool| RESOLVESTORAGE[Resolve File List]
    CHECKPOOL -->|AssetPool| RESOLVEASSET[Resolve Assets by Type]
    CHECKPOOL -->|Direct Spec| BUILDJOB

    RESOLVETARGET --> BUILDJOB[Build AttackSpec]
    RESOLVESTORAGE --> BUILDJOB
    RESOLVEASSET --> BUILDJOB

    BUILDJOB --> BUILDCMD[Build Command<br/>from Template]
    BUILDCMD --> CHECKPERIODIC{Periodic?}

    CHECKPERIODIC -->|Yes| CRONJOB[Create CronJob]
    CHECKPERIODIC -->|No| JOB[Create Job]

    CRONJOB --> BUILDPOD[Build PodSpec]
    JOB --> BUILDPOD

    BUILDPOD --> CHECKDEBUG{Debug<br/>Mode?}

    CHECKDEBUG -->|Yes| STDOUTLOG[stdout logging only]
    CHECKDEBUG -->|No| SIDECAR[Add FluentBit Sidecar]

    STDOUTLOG --> VOLUMES
    SIDECAR --> VOLUMES[Add Volumes<br/>input, output, fluent-bit-config]

    VOLUMES --> CHECKREV{Reverse Shell<br/>Enabled?}
    CHECKREV -->|Yes| SERVICE[Create Service<br/>ReverseShellHandler]
    CHECKREV -->|No| DEPLOY

    SERVICE --> DEPLOY[Deploy to K8s]
    DEPLOY --> UPDATESTATUS[Update Resource Status]
    UPDATESTATUS --> DONE([Reconciliation Complete])
```

## Pod Architecture

```mermaid
graph TB
    subgraph "Pod"
        subgraph "Main Container"
            TOOL[Security Tool<br/>nmap/metasploit/etc<br/>Executes command from template]
        end

        subgraph "Sidecar Container (optional)"
            FLUENTBIT[FluentBit<br/>Log Shipper<br/>Ships to Loki]
        end

        subgraph "Volumes"
            INPUT[input EmptyDir<br/>Downloaded config files]
            OUTPUT[output EmptyDir<br/>Shared logs]
            FLUENTCONFIG[fluent-bit-config ConfigMap<br/>FluentBit configuration]
        end
    end

    subgraph "External Resources"
        LOKI[Loki<br/>Log Aggregation]
        SERVICE[Service<br/>Optional - Reverse Shell]
        CONFIGMAPS_EXT[ConfigMaps<br/>Tool Specs]
    end

    CONFIGMAPS_EXT -->|tool config| TOOL
    TOOL -->|reads from| INPUT
    TOOL -->|writes to| OUTPUT
    FLUENTBIT -->|reads from| OUTPUT
    FLUENTBIT -->|reads config from| FLUENTCONFIG
    FLUENTBIT -->|ships logs to| LOKI
    SERVICE -->|exposes port from| TOOL
```

## CRD Relationships

```mermaid
erDiagram
    EXPLOIT ||--o| TARGETPOOL : "references"
    EXPLOIT ||--o| STORAGEPOOL : "references"
    EXPLOIT ||--|| SERVICE : "creates"

    ENUMERATION ||--o| TARGETPOOL : "references"
    ENUMERATION ||--o| STORAGEPOOL : "references"

    OSINT ||--o| ASSETPOOL : "references"
    OSINT ||--o| STORAGEPOOL : "references"

    LIVENESS ||--o| TARGETPOOL : "references"

    EXPLOIT ||--|| JOB : "creates"
    ENUMERATION ||--|| JOB : "creates"
    OSINT ||--|| JOB : "creates"
    LIVENESS ||--|| JOB : "creates"

    JOB ||--|| POD : "spawns"

    TARGETPOOL {
        string name
        string namespace
        array targetSpec
        int port
    }

    STORAGEPOOL {
        string name
        string namespace
        array files
    }

    ASSETPOOL {
        string name
        string namespace
        map asset
    }

    EXPLOIT {
        string tool
        string module
        string payload
        bool reverseShell
        string targetPool
        string storagePool
        bool periodic
        string schedule
    }

    ENUMERATION {
        string tool
        string targetPool
        string storagePool
        bool periodic
        string schedule
        int port
    }

    OSINT {
        string tool
        string assetPool
        string storagePool
        bool periodic
        string schedule
    }

    LIVENESS {
        string tool
        string targetPool
        bool periodic
        string schedule
    }

    JOB {
        string jobName
        string state
        timestamp lastExecuted
    }

    POD {
        array containers
        array volumes
    }

    SERVICE {
        string type
        int port
    }
```

## Configuration & Template System

```mermaid
flowchart LR
    subgraph "Configuration Sources"
        CM1[ConfigMap 1<br/>label: kentra.sh/resource-type=tool-specs]
        CM2[ConfigMap 2<br/>label: kentra.sh/resource-type=tool-specs]
        CM3[ConfigMap N<br/>label: kentra.sh/resource-type=tool-specs]
    end

    subgraph "ToolsConfigurator"
        LOADER[ConfigMap Loader<br/>List & Watch ConfigMaps]
        PARSER[YAML Parser<br/>Parse tools key]
        CACHE[Thread-Safe Cache<br/>sync.RWMutex]
    end

    subgraph "Tool Spec Structure"
        IMAGE[Image: ghcr.io/tool:tag]
        TEMPLATE[Command Template<br/>Go text/template]
        CAPS[Linux Capabilities]
    end

    subgraph "Template Execution"
        VARS[Template Variables:<br/>.Target.endpoint .Target.port<br/>.Args .Module .Payload<br/>.Item.TYPE .Files]
        EXECUTE[Execute Template]
        CMD[Final Command []string]
    end

    subgraph "Pod Container"
        CONTAINER[Container Spec<br/>Image + Command + Capabilities]
    end

    CM1 --> LOADER
    CM2 --> LOADER
    CM3 --> LOADER

    LOADER --> PARSER
    PARSER --> CACHE

    CACHE --> IMAGE
    CACHE --> TEMPLATE
    CACHE --> CAPS

    TEMPLATE --> VARS
    VARS --> EXECUTE
    EXECUTE --> CMD

    IMAGE --> CONTAINER
    CMD --> CONTAINER
    CAPS --> CONTAINER
```

## Controller Pattern Implementation

```mermaid
classDiagram
    class Reconciler {
        <<interface>>
        +Reconcile(ctx, req) Result
    }

    class BaseAttackReconciler {
        +Client
        +Scheme
        +ToolsConfigurator
        +ReconcileAttack(ctx, attack, factory)
        +ValidateNamespace(namespace)
        +EnsureLabels(attack)
    }

    class JobFactory {
        <<interface>>
        +ReconcileJobs(ctx) Result
    }

    class ExploitReconciler {
        +Reconcile(ctx, req) Result
    }

    class ExploitJobFactory {
        +exploit Exploit
        +ReconcileJobs(ctx) Result
        -buildAttackSpec() AttackSpec
    }

    class EnumerationReconciler {
        +Reconcile(ctx, req) Result
    }

    class EnumerationJobFactory {
        +enumeration Enumeration
        +ReconcileJobs(ctx) Result
    }

    class OsintReconciler {
        +Reconcile(ctx, req) Result
    }

    class OsintJobFactory {
        +osint Osint
        +ReconcileJobs(ctx) Result
    }

    class LivenessReconciler {
        +Reconcile(ctx, req) Result
    }

    class LivenessJobFactory {
        +liveness Liveness
        +ReconcileJobs(ctx) Result
    }

    class BasePoolReconciler {
        +Client
        +Scheme
        +ReconcilePool(ctx, pool, updater)
        +ValidateNamespace(namespace)
        +EnsureLabels(pool)
    }

    class PoolStatusUpdater {
        <<interface>>
        +UpdateStatus(ctx) Result
    }

    class TargetPoolReconciler {
        +Reconcile(ctx, req) Result
    }

    class StoragePoolReconciler {
        +Reconcile(ctx, req) Result
    }

    class AssetPoolReconciler {
        +Reconcile(ctx, req) Result
    }

    Reconciler <|.. ExploitReconciler
    Reconciler <|.. EnumerationReconciler
    Reconciler <|.. OsintReconciler
    Reconciler <|.. LivenessReconciler
    Reconciler <|.. TargetPoolReconciler
    Reconciler <|.. StoragePoolReconciler
    Reconciler <|.. AssetPoolReconciler

    BaseAttackReconciler <|-- ExploitReconciler
    BaseAttackReconciler <|-- EnumerationReconciler
    BaseAttackReconciler <|-- OsintReconciler
    BaseAttackReconciler <|-- LivenessReconciler

    JobFactory <|.. ExploitJobFactory
    JobFactory <|.. EnumerationJobFactory
    JobFactory <|.. OsintJobFactory
    JobFactory <|.. LivenessJobFactory

    ExploitReconciler --> ExploitJobFactory
    EnumerationReconciler --> EnumerationJobFactory
    OsintReconciler --> OsintJobFactory
    LivenessReconciler --> LivenessJobFactory

    BasePoolReconciler <|-- TargetPoolReconciler
    BasePoolReconciler <|-- StoragePoolReconciler
    BasePoolReconciler <|-- AssetPoolReconciler

    TargetPoolReconciler --> PoolStatusUpdater
    StoragePoolReconciler --> PoolStatusUpdater
    AssetPoolReconciler --> PoolStatusUpdater
```

## Data Flow: Enumeration Example

```mermaid
sequenceDiagram
    autonumber
    participant User
    participant K8s
    participant EnumCtrl as EnumerationReconciler
    participant Base as BaseAttackReconciler
    participant Factory as EnumerationJobFactory
    participant Resolver as PoolResolver
    participant Config as ToolsConfigurator
    participant JobBuilder
    participant Utils as Utilities

    User->>K8s: Create Enumeration CR
    K8s->>EnumCtrl: Reconcile Event

    EnumCtrl->>Base: ReconcileAttack(enumeration, factory)
    Base->>Base: ValidateNamespace()
    Base->>Base: EnsureLabels()
    Base->>Config: LoadConfig(tool)
    Config-->>Base: Tool Spec

    Base->>Factory: ReconcileJobs()

    Factory->>Resolver: ResolveTargetPool
    Resolver->>K8s: Get TargetPool
    Resolver->>Utils: ExpandCIDR & ExpandPortRange
    Utils-->>Resolver: Expanded IPs and Ports
    Resolver-->>Factory: []ResolvedTarget

    Factory->>Resolver: ResolveStoragePool
    Resolver->>K8s: Get StoragePool
    Resolver-->>Factory: File list

    Factory->>Config: BuildCommand(tool, target, port)
    Config-->>Factory: Templated command []string

    Factory->>JobBuilder: ReconcileJob(AttackSpec)
    JobBuilder->>JobBuilder: BuildPodSpec with PodBuilder
    JobBuilder->>K8s: Create Job/CronJob

    JobBuilder-->>Factory: Status update
    Factory-->>EnumCtrl: Update Status
    EnumCtrl->>K8s: Update Enumeration Status

    K8s->>K8s: Job spawns Pod
    Note over K8s: Main: Runs tool with templated command
    Note over K8s: Sidecar: Ships logs to Loki (if not debug)
```

## Module Organization

```mermaid
mindmap
    root((Kentra))
        cmd
            main.go
        api_v1alpha1[api/v1alpha1]
            AttackCRDs[Attack CRDs]
                exploit_types.go
                enumeration_types.go
                osint_types.go
                liveness_types.go
            PoolCRDs[Pool CRDs]
                targetpool_types.go
                storagepool_types.go
                assetpool_types.go
            Webhooks
                *_webhook.go
        internal_controller[internal/controller]
            base
                attack_reconciler.go
                pool_reconciler.go
            attacks
                enumeration_controller.go
                exploit_controller.go
                liveness_controller.go
                osint_controller.go
            pools
                assetpool_controller.go
                storagepool_controller.go
                targetpool_controller.go
            jobs
                job_builder.go
                attack_spec.go
            pods
                pod_builder.go
            config
                config.go (ToolsConfigurator)
            resolvers
                pool_resolver.go
            services
                reverse_shell.go
            utils
                converters.go
                network.go
        test
            e2e
                attack_interface.go
                enumeration_test.go
                liveness_test.go
                osint_test.go
            utils
                testutils.go
```

## Pool Resolution Flow

```mermaid
flowchart TD
    START([JobFactory needs resolved pools])

    START --> TARGET{Has<br/>TargetPool?}
    TARGET -->|Yes| RESOLVE_TARGET[PoolResolver.ResolveTargetPool]
    TARGET -->|No| STORAGE

    RESOLVE_TARGET --> EXPAND_CIDR[Expand CIDR ranges<br/>utils.ExpandCIDR]
    EXPAND_CIDR --> EXPAND_PORT[Expand port ranges<br/>utils.ExpandPortRange]
    EXPAND_PORT --> CROSS_PRODUCT[Cross-product: IPs × Ports]
    CROSS_PRODUCT --> STORAGE

    STORAGE{Has<br/>StoragePool?}
    STORAGE -->|Yes| RESOLVE_STORAGE[PoolResolver.ResolveStoragePool]
    STORAGE -->|No| ASSET

    RESOLVE_STORAGE --> GET_FILES[Get Files list from StoragePool.Spec]
    GET_FILES --> ASSET

    ASSET{Has<br/>AssetPool?}
    ASSET -->|Yes| RESOLVE_ASSET[PoolResolver.ResolveAssetPool]
    ASSET -->|No| BUILD_SPEC

    RESOLVE_ASSET --> MAP_ASSETS[Map assets by type<br/>map string to string<br/>map string to array]
    MAP_ASSETS --> BUILD_SPEC

    BUILD_SPEC[Build AttackSpec with resolved data]
    BUILD_SPEC --> DONE([Return AttackSpec])
```
