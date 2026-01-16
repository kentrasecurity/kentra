package main

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// Webhooks are like gatekeepers in Kubernetes
// They validate and modify resources during creation or updates
// it avoids invalid or harmful configurations from being applied to the cluster

func registerWebhooks(mgr ctrl.Manager) {
	webhooks := []struct {
		name  string
		setup WebhookSetup
	}{
		{"Enumeration", &securityv1alpha1.Enumeration{}},
		{"Liveness", &securityv1alpha1.Liveness{}},
		{"SecurityAttack", &securityv1alpha1.SecurityAttack{}},
		{"Osint", &securityv1alpha1.Osint{}},
		{"StoragePool", &securityv1alpha1.StoragePool{}},
		{"TargetPool", &securityv1alpha1.TargetPool{}},
		{"AssetPool", &securityv1alpha1.AssetPool{}},
	}

	for _, wh := range webhooks {
		if err := wh.setup.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", wh.name)
			os.Exit(1)
		}
	}

	setupLog.Info("Webhooks registered successfully")
}
