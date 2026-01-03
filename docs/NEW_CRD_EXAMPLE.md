# Creation of a CRD Security Type + OSINT Case

For these templates, substitute <Resource> with the name that you want to give to the new resource. In the use case we are going to substitute it with "Osint".

## 0. Define your attack
Choose a CLI tool and the type of target that it uses.

## 1. Define API Types

In the folder `api/v1alpha1/` there are the various Types that define the Kttack suite and that extends the Kubernetes basic ones.


1) Create a file `<name_of_the_type_of_attack>_types.go`.
2) Add the package `v1alpha1` and import the `k8s.io/apimachinery/pkg/apis/meta/v1` api

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
```

3) If you need it, define an environmental variable. This means that your new CRD will accept an Environmental variable defined by the user in the yaml file.
```go
// EnvVar represents an environment variable
type EnvVar struct {
//...
}
```

4) Define the desired state of the attack. The desired state is composed of a list of "properties", each one defined like 
   ``` Target <type_of_the_feature> `<type_of_the_output>:"<type_of_the_input>,<properties_of_the_input>"` ```

```go
// <Resource>Spec defines the desired state of <Resource>
type <Resource>Spec struct {
    Target        string   `json:"target,omitempty"`
    TargetPool   string   `json:"targetPool,omitempty"`
    Tool          string   `json:"tool"`
    Periodic      bool     `json:"periodic,omitempty"`
    Schedule      string   `json:"schedule,omitempty"`
    HTTPProxy     string   `json:"httpProxy,omitempty"`
    AdditionalEnv []EnvVar `json:"additionalEnv,omitempty"`
    Args          []string `json:"args,omitempty"`
    Debug         bool     `json:"debug,omitempty"`
    Port          string   `json:"port,omitempty"`
    StoragePool  string   `json:"storagePool,omitempty"`
}
```

5) Define the actual status of the attack. This follows the same logic and syntax of the desired state
```go
// <Resource>Status defines the observed state of <Resource>
type <Resource>Status struct {
    LastExecuted   string `json:"lastExecuted,omitempty"`
    State          string `json:"state,omitempty"`
    ResolvedTarget string `json:"resolvedTarget,omitempty"`
    ResolvedPort   string `json:"resolvedPort,omitempty"`
}
```

6) Define the schema of the new type. Start with defining metav1.TypeMeta and metav1.ObjectMeta with the syntax ```metav1.ObjectMeta `<type_of_the_input>,<properties_of_the_input>` ```. Then put the desired state (Spec) and the status.

```go
// <Resource> is the Schema for the <Resource>s API
type <Resource> struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              <Resource>Spec   `json:"spec,omitempty"`
    Status            <Resource>Status `json:"status,omitempty"`
}
```

7) Define the list of type of this attack.
```go
// <Resource>List contains a list of <Resource>
type <Resource>List struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []<Resource> `json:"items"`
}
```

8) Define the init function. This tells the controller to register the ney type and the list of types.
```go
func init() {
	SchemeBuilder.Register(&<Resource>{}, &<Resource>List{})
}
```

## 2. Create Reconciler
Create the `internal/controller/<resource>_controller.go` file. There is no template for this, it is dependent on the logic of the attack.



## 3. Register with Manager
In the file `cmd/main.go` you have to add to the main function after other controllers:

```go
// Create ToolsConfigurator for <Resource> controller
<resource>Configurator := controller.NewToolsConfigurator(mgr.GetClient(), controllerNamespace)

if err := (&controller.<Resource>Reconciler{
    Client:       mgr.GetClient(),
    Scheme:       mgr.GetScheme(),
    Configurator: <resource>Configurator,
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "<Resource>")
    os.Exit(1)
}
```


## 4. Generate CRD
Position yourself in the root folder of the project and run
```go
make manifest
```
This runs the controller-gen tool to generate CustomResourceDefinition YAML files `kentra.sh_<resource>s.yaml` in `config/crd/bases/`, RBAC manifests for the controller and WebhookConfiguration files (if applicable).

```go
make generate 
```
This ...

In the file `config/crd/kustomization.yaml` add `- bases/kentra.sh_<resource>.yaml` under resources.
```go
make install
```
This installs the new resources on the kbernetes cluster.

If you want to run the controller locally:
```go
make run
```

Optionally, you could run all of the previous commands with:
```sh
make all-crd #without make run
make all-crd-run #with make run
```

## 5. Test the new CRD
- if needed, create the targetpool under `config/samples/targetpools`
- create the new attack under `config/samples/attacks`
- create the targetpool and the attack in the cluster: `kubectl apply -f config/samples/targetpools/kentra_v1alpha1_targetpool_<resource>.yaml` , `kubectl apply -f config/samples/attacks/kentra_v1alpha1_<resource>.yaml`  
- add in the configmap `config/default/kentra-tool-specs.yaml` the new tool in `data.tools` like:
  ```yaml
    <tool>:
      type: "<resource>"
      image: "<tool_image>"
      commandTemplate: "<tool> {{.Args}} {{.Target}}"
      capabilities:
        add: []
  ```


# Case Study: OSINT Resource

## Attack Definition
In this case we are going to use the [Sherlock CLI](https://github.com/sherlock-project/sherlock) which comes with a docker image.
The general use of this command is:
```sh
sherlock user1 [user2 user3]
```
Therefore, the target is a list of usernames.

Summary:
    Tool: Sherlock CLI (username enumeration across social media platforms)
    Command format: `sherlock user1 [user2 user3]`
    Target: List of usernames

### Implementation
File: `api/v1alpha1/osint_types.go`
The OsintSpec includes standard fields (Target, Tool, Periodic, etc.) plus optional Category field for classifying OSINT operations.
File: `internal/controller/osint_controller.go`
The OsintReconciler follows the standard reconciliation pattern: loads tool configurations, resolves TargetPool/StoragePool references, validates inputs, creates Jobs or CronJobs based on the Periodic flag, and updates status.
Registration in `cmd/main.go`:

```go
osintConfigurator := controller.NewToolsConfigurator(mgr.GetClient(), controllerNamespace)

if err := (&controller.OsintReconciler{
    Client:       mgr.GetClient(),
    Scheme:       mgr.GetScheme(),
    Configurator: osintConfigurator,
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "Osint")
    os.Exit(1)
}
```

Usage example:

```yaml
apiVersion: kentra.sh/v1alpha1
kind: Osint
metadata:
  name: sherlock-scan
spec:
  tool: sherlock
  target: "johndoe janedoe"
  category: username-enumeration
  periodic: false
```

Build and run the controller:
```sh
make all-crd-run #with make run
```

Test it:
- create the targetpool under `config/samples/targetpools`

```yaml
---
apiVersion: kentra.sh/v1alpha1
kind: TargetPool
metadata:
  name: osint-targets
  namespace: kentra-system
spec:
  target: "patrickdifazio"
  description: "Target group for osint Patrick Di Fazio"
```

- create the new attack under `config/samples/attacks`
```yaml
---
apiVersion: kentra.sh/v1alpha1
kind: Osint
metadata:
  name: osint
  namespace: kentra-system
spec:
  tool: sherlock
  targetPool: osint-targets
  category: username-enumeration
  periodic: false
```

- create the targetpool and the attack in the cluster: `kubectl apply -f config/samples/targetpools/kentra_v1alpha1_targetpool_osint.yaml` , `kubectl apply -f config/samples/attacks/kentra_v1alpha1_osint.yaml`  
- 
- add in the configmap `config/default/kentra-tool-specs.yaml` the new tool in `data.tools` like:
  ```yaml
    sherlock:
      type: "osint"
      image: "sherlock/sherlock:0.16.0"
      commandTemplate: "sherlock {{.Args}} {{.Target}}"
      capabilities:
        add: []
  ```