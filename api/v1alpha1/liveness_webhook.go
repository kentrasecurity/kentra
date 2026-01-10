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

package v1alpha1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var livenesslog = logf.Log.WithName("liveness-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *Liveness) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&LivenessValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-kentra-sh-v1alpha1-liveness,mutating=false,failurePolicy=fail,sideEffects=None,groups=kentra.sh,resources=livenesses,verbs=create;update,versions=v1alpha1,name=vliveness.kb.io,admissionReviewVersions=v1

// LivenessValidator validates Liveness resources
type LivenessValidator struct {
	Client client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *LivenessValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	liveness := obj.(*Liveness)
	livenesslog.Info("validate create", "name", liveness.Name)

	return v.validateNamespace(ctx, liveness.Namespace)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *LivenessValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	liveness := newObj.(*Liveness)
	livenesslog.Info("validate update", "name", liveness.Name)

	return v.validateNamespace(ctx, liveness.Namespace)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *LivenessValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	liveness := obj.(*Liveness)
	livenesslog.Info("validate delete", "name", liveness.Name)

	// No validation needed for delete
	return nil, nil
}

// validateNamespace checks if the namespace has the managed-by-kentra annotation
func (v *LivenessValidator) validateNamespace(ctx context.Context, namespace string) (admission.Warnings, error) {
	ns := &corev1.Namespace{}
	if err := v.Client.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		return nil, fmt.Errorf("failed to get namespace %s: %w", namespace, err)
	}

	// Check for managed-by-kentra annotation
	if annotations := ns.GetAnnotations(); annotations != nil {
		if _, ok := annotations["managed-by-kentra"]; ok {
			return nil, nil
		}
	}

	return nil, fmt.Errorf("namespace '%s' is not managed by Kentra (missing 'managed-by-kentra' annotation)", namespace)
}
