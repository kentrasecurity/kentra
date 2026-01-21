/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// package-level variables that are initialized when the package loads
var (
	scheme   = runtime.NewScheme()        //Creates a new Kubernetes Scheme object that maps Go types to Kubernetes API Group-Version-Kinds (GVK)
	setupLog = ctrl.Log.WithName("setup") // Creates a logger specifically for setup/initialization code
)

// init() runs before `main()` executes
// Must() panics if the function passed to it returns an error
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))   // Registers all built-in Kubernetes types with the scheme
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme)) // Registers all custom securityv1alpha1 types with the scheme (our CRDs)
}

func main() {
	// Parse command-line flags and set up logger
	cfg := parseFlags()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&cfg.zapOpts)))

	// Create the controller manager
	mgr := createManager(cfg, scheme)
	controllerNamespace := getControllerNamespace() //reads POD_NAMESPACE variable (this variable is set in the deployment manifest and it is loaded into the pod's controller env when deployed)

	// Register all controllers
	registerControllers(mgr, controllerNamespace)

	// Register webhooks if enabled
	if os.Getenv("ENABLE_WEBHOOKS") == "true" {
		registerWebhooks(mgr)
	}

	// Setup health checks
	setupHealthChecks(mgr)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getControllerNamespace() string {
	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		ns = "kentra-system"
		setupLog.Info("POD_NAMESPACE not set, using default", "namespace", ns)
	} else {
		setupLog.Info("Using controller namespace", "namespace", ns)
	}
	return ns
}
