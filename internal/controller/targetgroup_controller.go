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

// TargetPoolReconciler reconciles a TargetPool object
type TargetPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools/finalizers,verbs=update

// Reconcile implements reconciliation for TargetPool resources
func (r *TargetPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the TargetPool resource
	tg := &securityv1alpha1.TargetPool{}
	if err := r.Get(ctx, req.NamespacedName, tg); err != nil {
		if errors.IsNotFound(err) {
			log.Info("TargetPool resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get TargetPool")
		return ctrl.Result{}, err
	}

	// Check if namespace is managed by Kentra
	isManaged, err := isNamespaceManagedByKentra(ctx, r.Client, tg.Namespace)
	if err != nil {
		log.Error(err, "Failed to check if namespace is managed by Kentra", "namespace", tg.Namespace)
		return ctrl.Result{}, err
	}
	if !isManaged {
		log.Error(fmt.Errorf("namespace not managed by Kentra"), "Cannot create TargetPool in namespace without 'managed-by-kentra' annotation", "namespace", tg.Namespace)
		return ctrl.Result{}, fmt.Errorf("namespace %s is not managed by Kentra (missing 'managed-by-kentra' annotation)", tg.Namespace)
	}

	// Ensure labels are set
	if tg.Labels == nil {
		tg.Labels = make(map[string]string)
	}
	needsUpdate := false
	if tg.Labels["kentra.sh/resource-type"] != "target" {
		tg.Labels["kentra.sh/resource-type"] = "target"
		needsUpdate = true
	}

	// Update the resource if labels were modified
	if needsUpdate {
		if err := r.Update(ctx, tg); err != nil {
			log.Error(err, "Failed to update TargetPool labels")
			return ctrl.Result{}, err
		}
	}

	// Update the status with current timestamp
	tg.Status.LastUpdated = time.Now().Format(time.RFC3339)
	tg.Status.ObservedGeneration = tg.ObjectMeta.Generation

	if err := r.Status().Update(ctx, tg); err != nil {
		log.Error(err, "Failed to update TargetPool status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled TargetPool", "TargetPool", tg.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.TargetPool{}).
		Complete(r)
}
