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

package pools

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	"github.com/kentrasecurity/kentra/internal/controller/base"
)

// StoragePoolReconciler reconciles a StoragePool object
type StoragePoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kentra.sh,resources=storagepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=storagepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=storagepools/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list

func (r *StoragePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	baseReconciler := &base.BasePoolReconciler{
		Client:       r.Client,
		Scheme:       r.Scheme,
		ResourceType: "storage",
	}

	storagePool := &securityv1alpha1.StoragePool{}
	updater := &StoragePoolStatusUpdater{client: r.Client}

	return baseReconciler.ReconcilePool(ctx, req, storagePool, updater)
}

// StoragePoolStatusUpdater updates StoragePool-specific status
type StoragePoolStatusUpdater struct {
	client client.Client
}

func (u *StoragePoolStatusUpdater) UpdateStatus(ctx context.Context, resource base.PoolResource) error {
	sp := resource.(*securityv1alpha1.StoragePool)

	// Update common fields
	sp.Status.LastUpdated = time.Now().Format(time.RFC3339)
	sp.Status.ObservedGeneration = sp.Generation
	sp.Status.ItemCount = len(sp.Spec.Files)

	return nil
}

func (r *StoragePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.StoragePool{}).
		Complete(r)
}
