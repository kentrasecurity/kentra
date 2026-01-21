package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

// Reconciler interface for controllers that need setup
type Reconciler interface {
	SetupWithManager(mgr ctrl.Manager) error
}

// WebhookSetup interface for resources with webhooks
type WebhookSetup interface {
	SetupWebhookWithManager(mgr ctrl.Manager) error
}
