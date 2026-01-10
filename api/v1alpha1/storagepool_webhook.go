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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var storagepoollog = logf.Log.WithName("storagepool-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *StoragePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&StoragePoolValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-kentra-sh-v1alpha1-storagepool,mutating=false,failurePolicy=fail,sideEffects=None,groups=kentra.sh,resources=storagepools,verbs=create;update,versions=v1alpha1,name=vstoragepool.kb.io,admissionReviewVersions=v1

// StoragePoolValidator validates StoragePool resources
// +kubebuilder:object:generate=false
type StoragePoolValidator struct {
	Client client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *StoragePoolValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sp := obj.(*StoragePool)
	storagepoollog.Info("validate create", "name", sp.Name)

	return v.validateNamespace(ctx, sp.Namespace)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *StoragePoolValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	sp := newObj.(*StoragePool)
	storagepoollog.Info("validate update", "name", sp.Name)

	return v.validateNamespace(ctx, sp.Namespace)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *StoragePoolValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sp := obj.(*StoragePool)
	storagepoollog.Info("validate delete", "name", sp.Name)

	// No validation needed for delete
	return nil, nil
}

// validateNamespace checks if the namespace has the managed-by-kentra annotation
func (v *StoragePoolValidator) validateNamespace(ctx context.Context, namespace string) (admission.Warnings, error) {
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
