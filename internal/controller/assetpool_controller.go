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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// AssetPoolReconciler reconciles an AssetPool object
type AssetPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kentra.sh,resources=assetpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=assetpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=assetpools/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list

// Reconcile implements reconciliation for AssetPool resources
func (r *AssetPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the AssetPool resource
	ap := &securityv1alpha1.AssetPool{}
	if err := r.Get(ctx, req.NamespacedName, ap); err != nil {
		if errors.IsNotFound(err) {
			log.Info("AssetPool resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get AssetPool")
		return ctrl.Result{}, err
	}

	// Check if namespace is managed by Kentra
	isManaged, err := isNamespaceManagedByKentra(ctx, r.Client, ap.Namespace)
	if err != nil {
		log.Error(err, "Failed to check if namespace is managed by Kentra", "namespace", ap.Namespace)
		return ctrl.Result{}, err
	}
	if !isManaged {
		log.Error(fmt.Errorf("namespace not managed by Kentra"), "Cannot create AssetPool in namespace without 'managed-by-kentra' annotation", "namespace", ap.Namespace)
		return ctrl.Result{}, fmt.Errorf("namespace %s is not managed by Kentra (missing 'managed-by-kentra' annotation)", ap.Namespace)
	}

	// Ensure labels are set
	if ap.Labels == nil {
		ap.Labels = make(map[string]string)
	}
	needsUpdate := false
	if ap.Labels["kentra.sh/resource-type"] != "asset" {
		ap.Labels["kentra.sh/resource-type"] = "asset"
		needsUpdate = true
	}

	// Update the resource if labels were modified
	if needsUpdate {
		if err := r.Update(ctx, ap); err != nil {
			log.Error(err, "Failed to update AssetPool labels")
			return ctrl.Result{}, err
		}
	}

	// Calculate counts
	itemCount := len(ap.Spec.Items)
	groupCount := len(ap.Spec.Groups)
	totalAssetSets := 0

	for _, group := range ap.Spec.Groups {
		if len(group.AssetSets) > 0 {
			totalAssetSets += len(group.AssetSets)
		} else if len(group.Assets) > 0 {
			totalAssetSets += 1 // Legacy single asset set
		}
	}

	// Update status
	ap.Status.ItemCount = itemCount
	ap.Status.GroupCount = groupCount
	ap.Status.TotalAssetSets = totalAssetSets
	ap.Status.LastUpdated = time.Now().Format(time.RFC3339)
	ap.Status.ObservedGeneration = ap.Generation

	if err := r.Status().Update(ctx, ap); err != nil {
		log.Error(err, "Failed to update AssetPool status")
		return ctrl.Result{}, err
	}

	log.Info("AssetPool reconciled successfully",
		"AssetPool", ap.Name,
		"ItemCount", itemCount,
		"GroupCount", groupCount,
		"TotalAssetSets", totalAssetSets)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AssetPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.AssetPool{}).
		Complete(r)
}
