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

package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kttack/kttack/api/v1alpha1"
)

// StoragePoolReconciler reconciles a StoragePool object
type StoragePoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kttack.io,resources=storagepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=storagepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=storagepools/finalizers,verbs=update

// Reconcile implements reconciliation for StoragePool resources
func (r *StoragePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StoragePool resource
	sg := &securityv1alpha1.StoragePool{}
	if err := r.Get(ctx, req.NamespacedName, sg); err != nil {
		if errors.IsNotFound(err) {
			log.Info("StoragePool resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get StoragePool")
		return ctrl.Result{}, err
	}

	// Ensure labels are set
	if sg.Labels == nil {
		sg.Labels = make(map[string]string)
	}
	needsUpdate := false
	if sg.Labels["kttack-resource-type"] != "storage" {
		sg.Labels["kttack-resource-type"] = "storage"
		needsUpdate = true
	}

	// Update the resource if labels were modified
	if needsUpdate {
		if err := r.Update(ctx, sg); err != nil {
			log.Error(err, "Failed to update StoragePool labels")
			return ctrl.Result{}, err
		}
	}

	// Update status with file count and sync time
	sg.Status.FileCount = len(sg.Spec.Files)
	sg.Status.LastSyncTime = time.Now().Format(time.RFC3339)
	sg.Status.ObservedGeneration = sg.Generation

	// Update status
	if err := r.Status().Update(ctx, sg); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("StoragePool reconciled successfully", "StoragePool", sg.Name, "FileCount", len(sg.Spec.Files))
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StoragePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.StoragePool{}).
		Complete(r)
}
