package main

import (
	"os"

	"github.com/kentrasecurity/kentra/internal/controller/attacks"
	"github.com/kentrasecurity/kentra/internal/controller/config"
	"github.com/kentrasecurity/kentra/internal/controller/pools"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ControllerConfig holds configuration for all types of controller
type ControllerConfig struct {
	Name              string
	ReconcilerFactory func(ctrl.Manager, string) Reconciler
}

func registerControllers(mgr ctrl.Manager, namespace string) {
	controllers := []ControllerConfig{
		{
			Name: "Osint",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				configurator := config.NewToolsConfigurator(mgr.GetClient(), ns)
				setupLog.Info("ToolsConfigurator created", "controller", "Osint")
				return &attacks.OsintReconciler{
					Client:              mgr.GetClient(),
					Scheme:              mgr.GetScheme(),
					Configurator:        configurator,
					ControllerNamespace: ns,
				}
			},
		},
		{
			Name: "Exploit",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				configurator := config.NewToolsConfigurator(mgr.GetClient(), ns)
				setupLog.Info("ToolsConfigurator created", "controller", "Exploit")
				return &attacks.ExploitReconciler{
					Client:              mgr.GetClient(),
					Scheme:              mgr.GetScheme(),
					Configurator:        configurator,
					ControllerNamespace: ns,
				}
			},
		},
		{
			Name: "Enumeration",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				configurator := config.NewToolsConfigurator(mgr.GetClient(), ns)
				setupLog.Info("ToolsConfigurator created", "controller", "Enumeration")
				return &attacks.EnumerationReconciler{
					Client:              mgr.GetClient(),
					Scheme:              mgr.GetScheme(),
					Configurator:        configurator,
					ControllerNamespace: ns,
				}
			},
		},
		{
			Name: "Liveness",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				configurator := config.NewToolsConfigurator(mgr.GetClient(), ns)
				setupLog.Info("ToolsConfigurator created", "controller", "Liveness")
				return &attacks.LivenessReconciler{
					Client:              mgr.GetClient(),
					Scheme:              mgr.GetScheme(),
					Configurator:        configurator,
					ControllerNamespace: ns,
				}
			},
		},
		{
			Name: "SecurityAttack",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				configurator := config.NewToolsConfigurator(mgr.GetClient(), ns)
				setupLog.Info("ToolsConfigurator created", "controller", "SecurityAttack")
				return &attacks.SecurityAttackReconciler{
					Client:              mgr.GetClient(),
					Scheme:              mgr.GetScheme(),
					Configurator:        configurator,
					ControllerNamespace: ns,
				}
			},
		},
		{
			Name: "TargetPool",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				return &pools.TargetPoolReconciler{
					Client: mgr.GetClient(),
					Scheme: mgr.GetScheme(),
				}
			},
		},
		{
			Name: "StoragePool",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				return &pools.StoragePoolReconciler{
					Client: mgr.GetClient(),
					Scheme: mgr.GetScheme(),
				}
			},
		},
		{
			Name: "AssetPool",
			ReconcilerFactory: func(mgr ctrl.Manager, ns string) Reconciler {
				return &pools.AssetPoolReconciler{
					Client: mgr.GetClient(),
					Scheme: mgr.GetScheme(),
				}
			},
		},
	}

	// Single registration loop for all controllers
	for _, cfg := range controllers {
		reconciler := cfg.ReconcilerFactory(mgr, namespace)
		if err := reconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", cfg.Name)
			os.Exit(1)
		}
	}
}
