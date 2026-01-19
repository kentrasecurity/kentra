# Creating a New Attack CRD (Refactored Architecture)

This guide shows how to add a new attack type to the refactored Kentra codebase.

**Estimated time**: 30-45 minutes  
**Lines of code**: ~70-100 (vs 250+ in old architecture)

---

## Prerequisites

Understand what your attack does:
- ✅ CLI tool name and image
- ✅ Target type (IP, domain, username, etc.)
- ✅ Command template
- ✅ Required arguments/options
- ✅ Special features (periodic, files, capabilities, etc.)

---

## Step 1: Define API Types (5 minutes)

**File**: `api/v1alpha1/<attack>_types.go`

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// <Attack>Spec defines the desired state
type <Attack>Spec struct {
    // Required fields
    Tool string `json:"tool"`
    
    // Target resolution (at least one required)
    Target     string `json:"target,omitempty"`
    TargetPool string `json:"targetPool,omitempty"`
    
    // Optional common fields
    Category      string   `json:"category,omitempty"`
    Args          []string `json:"args,omitempty"`
    HTTPProxy     string   `json:"httpProxy,omitempty"`
    AdditionalEnv []EnvVar `json:"additionalEnv,omitempty"`
    Debug         bool     `json:"debug,omitempty"`
    
    // Scheduling
    Periodic bool   `json:"periodic,omitempty"`
    Schedule string `json:"schedule,omitempty"`
    
    // Optional features
    Port        string `json:"port,omitempty"`
    StoragePool string `json:"storagePool,omitempty"`
    
    // Add attack-specific fields here if needed
    // CustomField string `json:"customField,omitempty"`
}

// <Attack>Status defines the observed state
type <Attack>Status struct {
    State          string `json:"state,omitempty"`
    LastExecuted   string `json:"lastExecuted,omitempty"`
    ResolvedTarget string `json:"resolvedTarget,omitempty"`
    ResolvedPort   string `json:"resolvedPort,omitempty"`
}

// <Attack> is the Schema for the API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=<shortname>
type <Attack> struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              <Attack>Spec   `json:"spec,omitempty"`
    Status            <Attack>Status `json:"status,omitempty"`
}

// <Attack>List contains a list of <Attack>
// +kubebuilder:object:root=true
type <Attack>List struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []<Attack> `json:"items"`
}

func init() {
    SchemeBuilder.Register(&<Attack>{}, &<Attack>List{})
}
```

**Example for OSINT**:
```go
type OsintSpec struct {
    Tool          string   `json:"tool"`
    Target        string   `json:"target,omitempty"`
    TargetPool    string   `json:"targetPool,omitempty"`
    AssetPool     string   `json:"assetPool,omitempty"`  // OSINT-specific
    Category      string   `json:"category,omitempty"`
    Args          []string `json:"args,omitempty"`
    HTTPProxy     string   `json:"httpProxy,omitempty"`
    AdditionalEnv []EnvVar `json:"additionalEnv,omitempty"`
    Debug         bool     `json:"debug,omitempty"`
    Periodic      bool     `json:"periodic,omitempty"`
    Schedule      string   `json:"schedule,omitempty"`
    Port          string   `json:"port,omitempty"`
    StoragePool   string   `json:"storagePool,omitempty"`
}
```

---

## Step 2: Create Controller (~70 lines)

**File**: `internal/controller/attacks/<attack>_controller.go`

### Template for Simple Attacks (Target-based)

```go
package attacks

import (
    "context"
    "fmt"

    batchv1 "k8s.io/api/batch/v1"
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"

    securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
    "github.com/kentrasecurity/kentra/internal/controller/base"
    "github.com/kentrasecurity/kentra/internal/controller/config"
    "github.com/kentrasecurity/kentra/internal/controller/jobs"
    "github.com/kentrasecurity/kentra/internal/controller/resolvers"
    "github.com/kentrasecurity/kentra/internal/controller/utils"
)

type <Attack>Reconciler struct {
    client.Client
    Scheme              *runtime.Scheme
    Configurator        *config.ToolsConfigurator
    ControllerNamespace string
}

//+kubebuilder:rbac:groups=kentra.sh,resources=<attacks>,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=<attacks>/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools;storagepools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets;namespaces,verbs=get;list;watch;create

func (r *<Attack>Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    baseReconciler := &base.BaseAttackReconciler{
        Client:              r.Client,
        Scheme:              r.Scheme,
        Configurator:        r.Configurator,
        ControllerNamespace: r.ControllerNamespace,
        ResourceType:        "<attack>",
    }

    attack := &securityv1alpha1.<Attack>{}
    factory := &<Attack>JobFactory{
        Client:              r.Client,
        Scheme:              r.Scheme,
        Configurator:        r.Configurator,
        ControllerNamespace: r.ControllerNamespace,
    }

    return baseReconciler.ReconcileAttack(ctx, req, attack, factory)
}

type <Attack>JobFactory struct {
    Client              client.Client
    Scheme              *runtime.Scheme
    Configurator        *config.ToolsConfigurator
    ControllerNamespace string
}

func (f *<Attack>JobFactory) ReconcileJobs(ctx context.Context, resource base.AttackResource) (ctrl.Result, error) {
    attack := resource.(*securityv1alpha1.<Attack>)
    resolver := resolvers.New(f.Client)

    // Resolve pools (customize based on what your attack needs)
    files, _ := resolver.ResolveStoragePool(ctx, attack.Spec.StoragePool, attack.Namespace)
    target, port := resolver.ResolveTargetWithPort(ctx, attack.Spec.TargetPool, attack.Spec.Target, attack.Spec.Port, attack.Namespace)

    if target == "" {
        return ctrl.Result{}, fmt.Errorf("no target specified")
    }

    attack.Status.ResolvedTarget = target
    attack.Status.ResolvedPort = port

    // Create job spec
    spec := &jobs.AttackSpec{
        Tool:          attack.Spec.Tool,
        Target:        target,
        Category:      attack.Spec.Category,
        Args:          attack.Spec.Args,
        HTTPProxy:     attack.Spec.HTTPProxy,
        AdditionalEnv: utils.ConvertEnvVars(attack.Spec.AdditionalEnv),
        Debug:         attack.Spec.Debug,
        Periodic:      attack.Spec.Periodic,
        Schedule:      attack.Spec.Schedule,
        Port:          port,
        Files:         files,
    }

    builder := &jobs.JobBuilder{
        Client:              f.Client,
        Scheme:              f.Scheme,
        Configurator:        f.Configurator,
        ControllerNamespace: f.ControllerNamespace,
        ResourceType:        "<attack>",
    }

    return builder.ReconcileJob(ctx, attack, attack.Name, spec, func(status *jobs.AttackStatus) {
        attack.Status.State = status.State
        attack.Status.LastExecuted = status.LastExecuted
    })
}

func (r *<Attack>Reconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&securityv1alpha1.<Attack>{}).
        Owns(&batchv1.Job{}).
        Owns(&batchv1.CronJob{}).
        Complete(r)
}
```

### Customization Points

**If your attack has special features**:

```go
// Asset-based (like OSINT with AssetPool)
if attack.Spec.AssetPool != "" {
    assets, _ := resolver.ResolveAssetPool(ctx, attack.Spec.AssetPool, attack.Namespace)
    // Create multiple jobs (see osint_controller.go for example)
}

// Custom options (like Exploit)
args := append([]string{}, attack.Spec.Args...)
for key, value := range attack.Spec.Options {
    args = append(args, fmt.Sprintf("%s=%s", key, value))
}

// Special capabilities (like Exploit)
spec.Capabilities = attack.Spec.Capabilities
spec.ReverseShell = attack.Spec.ReverseShell
```

---

## Step 3: Register Controller (2 minutes)

**File**: `cmd/controllers.go`

Add to the `controllers` slice in `registerControllers()`:

```go
{
    Name: "<Attack>",
    ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
        configurator := config.NewToolsConfigurator(mgr.GetClient(), ns)
        setupLog.Info("ToolsConfigurator created", "controller", "<Attack>")
        return &attacks.<Attack>Reconciler{
            Client:              mgr.GetClient(),
            Scheme:              mgr.GetScheme(),
            Configurator:        configurator,
            ControllerNamespace: ns,
        }
    },
},
```

---

## Step 4: Generate CRD (1 minute)

```bash
# Generate code
make generate

# Generate CRD manifests
make manifests

# Install CRDs in cluster
make install
```

**Add to `config/crd/kustomization.yaml`**:
```yaml
resources:
  - bases/kentra.sh_<attacks>.yaml
```

---

## Step 5: Configure Tool (2 minutes)

**File**: `config/default/kentra-tool-specs.yaml`

Add your tool configuration:

```yaml
data:
  tools: |
    <tool-name>:
      type: "<attack>"
      image: "<tool-image>:<version>"
      commandTemplate: "<tool-binary> {{.Args}} {{.Target}}"
      capabilities:
        add: []
```

**Example for Sherlock (OSINT)**:
```yaml
sherlock:
  type: "osint"
  image: "sherlock/sherlock:0.16.0"
  commandTemplate: "sherlock {{.Args}} {{.Target}}"
  capabilities:
    add: []
```

**Example with assets**:
```yaml
holehe:
  type: "osint"
  image: "megadose/holehe:latest"
  commandTemplate: "holehe {{.Item.email}}"
  assetTypeFlags:
    email: ""
```

---

## Step 6: Create Sample Resources (5 minutes)

### TargetPool (if needed)

**File**: `config/samples/targetpools/kentra_v1alpha1_targetpool_<attack>.yaml`

```yaml
apiVersion: kentra.sh/v1alpha1
kind: TargetPool
metadata:
  name: <attack>-targets
  namespace: kentra-system
spec:
  target: "example.com"
  port: "443"
  description: "Target pool for <attack> testing"
```

### Attack Resource

**File**: `config/samples/attacks/kentra_v1alpha1_<attack>.yaml`

```yaml
apiVersion: kentra.sh/v1alpha1
kind: <Attack>
metadata:
  name: <attack>-test
  namespace: kentra-system
spec:
  tool: <tool-name>
  targetPool: <attack>-targets
  category: <category>
  periodic: false
  debug: false
```

---

## Step 7: Test Your Attack (5 minutes)

### Manual Testing

```bash
# Apply target pool
kubectl apply -f config/samples/targetpools/kentra_v1alpha1_targetpool_<attack>.yaml

# Apply attack
kubectl apply -f config/samples/attacks/kentra_v1alpha1_<attack>.yaml

# Check status
kubectl get <attacks> -n kentra-system
kubectl describe <attack> <attack>-test -n kentra-system

# Check job
kubectl get jobs -n kentra-system
kubectl logs -f job/<attack>-test -n kentra-system

# Check results in Loki (if configured)
# Query: {namespace="kentra-system",job_name="<attack>-test"}
```

### Run Controller Locally (for development)

```bash
make run
```

---

## Step 8: Add to Helm Chart (10 minutes)

### 1. Copy CRD

```bash
cp config/crd/bases/kentra.sh_<attacks>.yaml helm/crds/<attack>-crd.yaml
```

### 2. Update RBAC

**File**: `helm/manager-rbac.yaml`

Add to each section (note the plural 's'):

```yaml
# Resources
- apiGroups:
    - kentra.sh
  resources:
    - <attacks>
  verbs:
    - create
    - delete
    - get
    - list
    - patch
    - update
    - watch

# Finalizers
- apiGroups:
    - kentra.sh
  resources:
    - <attacks>/finalizers
  verbs:
    - update

# Status
- apiGroups:
    - kentra.sh
  resources:
    - <attacks>/status
  verbs:
    - get
    - patch
    - update
```

### 3. Update Tool Specs ConfigMap

**File**: `helm/templates/kentra-tool-specs.yaml`

Add your tool configuration under `data.tools`.

---

## Step 9: Write Tests (Optional but Recommended)

**File**: `internal/controller/attacks/<attack>_controller_test.go`

```go
package attacks

import (
    "context"
    "testing"

    securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
    // ... imports
)

func Test<Attack>Reconciler_Simple(t *testing.T) {
    // Test basic reconciliation
}

func Test<Attack>JobFactory_TargetResolution(t *testing.T) {
    // Test target pool resolution
}
```

Run tests:
```bash
make test
```

---

## Complete Example: OSINT Attack

### 1. API Type (`api/v1alpha1/osint_types.go`)

```go
type OsintSpec struct {
    Tool          string   `json:"tool"`
    Target        string   `json:"target,omitempty"`
    TargetPool    string   `json:"targetPool,omitempty"`
    AssetPool     string   `json:"assetPool,omitempty"`
    Category      string   `json:"category,omitempty"`
    Args          []string `json:"args,omitempty"`
    HTTPProxy     string   `json:"httpProxy,omitempty"`
    AdditionalEnv []EnvVar `json:"additionalEnv,omitempty"`
    Debug         bool     `json:"debug,omitempty"`
    Periodic      bool     `json:"periodic,omitempty"`
    Schedule      string   `json:"schedule,omitempty"`
    Port          string   `json:"port,omitempty"`
    StoragePool   string   `json:"storagePool,omitempty"`
}
```

### 2. Controller (`internal/controller/attacks/osint_controller.go`)

See the refactored version - ~130 lines including AssetPool support!

### 3. Tool Config

```yaml
sherlock:
  type: "osint"
  image: "sherlock/sherlock:0.16.0"
  commandTemplate: "sherlock {{.Args}} {{.Target}}"
```

### 4. Sample Usage

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Osint
metadata:
  name: sherlock-scan
  namespace: kentra-system
spec:
  tool: sherlock
  target: "johndoe janedoe"
  category: username-enumeration
  periodic: false
```

---

## Architecture Benefits

### Old Way (250+ lines per controller)
- ❌ Duplicate namespace validation
- ❌ Duplicate label management
- ❌ Duplicate job building
- ❌ Duplicate pod spec building
- ❌ Hard to maintain

### New Way (~70-100 lines per controller)
- ✅ Reuses `BaseAttackReconciler` (namespace, labels, config)
- ✅ Reuses `JobBuilder` (job/cronjob creation)
- ✅ Reuses `PodBuilder` (pod spec creation)
- ✅ Reuses `PoolResolver` (target/storage/asset resolution)
- ✅ Only write attack-specific logic

---

## Common Patterns

### Pattern 1: Simple Target-Based Attack
Examples: Liveness, SecurityAttack

**What you write** (~70 lines):
- Resolve target from TargetPool
- Build AttackSpec
- Call JobBuilder

### Pattern 2: Target + Files Attack
Examples: Enumeration

**What you write** (~100 lines):
- Resolve target and storage pools
- Build AttackSpec with files
- Call JobBuilder

### Pattern 3: Multiple Jobs (Asset-Based)
Examples: Osint

**What you write** (~130 lines):
- Resolve AssetPool
- Create one job per asset group
- Handle multiple job tracking

### Pattern 4: Complex Attack with Services
Examples: Exploit

**What you write** (~140 lines):
- Resolve pools
- Create job
- Create LoadBalancer service for reverse shell

---

## Troubleshooting

### Compilation Errors

```bash
# Check for errors
go build ./internal/controller/attacks/<attack>_controller.go

# Common issues:
# - Missing imports
# - Wrong package name
# - Type mismatch (use utils.ConvertEnvVars for EnvVar)
```

### CRD Not Generated

```bash
# Ensure kubebuilder markers are present:
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

# Re-run generation
make generate
make manifests
```

### Controller Not Starting

```bash
# Check registration in cmd/controllers.go
# Verify all imports are correct
# Check logs: kubectl logs -n kentra-system deployment/kentra-controller-manager
```

---

## Checklist

- [ ] Created `api/v1alpha1/<attack>_types.go`
- [ ] Created `internal/controller/attacks/<attack>_controller.go` (~70-140 lines)
- [ ] Registered in `cmd/controllers.go`
- [ ] Ran `make generate && make manifests`
- [ ] Added to `config/crd/kustomization.yaml`
- [ ] Configured tool in `config/default/kentra-tool-specs.yaml`
- [ ] Created sample TargetPool
- [ ] Created sample Attack resource
- [ ] Tested manually
- [ ] Added CRD to `helm/crds/`
- [ ] Updated `helm/manager-rbac.yaml`
- [ ] Updated `helm/templates/kentra-tool-specs.yaml`
- [ ] (Optional) Wrote tests

---

## Summary

**Time to add new attack**: ~30-45 minutes  
**Lines of code**: ~70-140 lines (vs 250+ before)  
**Reused components**: BaseReconciler, JobBuilder, PodBuilder, PoolResolver, Utils  
**Maintainability**: High - change base components, all attacks benefit  

The refactored architecture makes adding new attacks **fast, simple, and maintainable**! 🚀
