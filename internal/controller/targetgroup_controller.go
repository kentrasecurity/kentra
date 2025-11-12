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

// TargetGroupReconciler reconciles a TargetGroup object
type TargetGroupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kttack.io,resources=targetgroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=targetgroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=targetgroups/finalizers,verbs=update

// Reconcile implements reconciliation for TargetGroup resources
func (r *TargetGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the TargetGroup resource
	tg := &securityv1alpha1.TargetGroup{}
	if err := r.Get(ctx, req.NamespacedName, tg); err != nil {
		if errors.IsNotFound(err) {
			log.Info("TargetGroup resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get TargetGroup")
		return ctrl.Result{}, err
	}

	// Update the status with current timestamp
	tg.Status.LastUpdated = time.Now().Format(time.RFC3339)
	tg.Status.ObservedGeneration = tg.ObjectMeta.Generation

	if err := r.Status().Update(ctx, tg); err != nil {
		log.Error(err, "Failed to update TargetGroup status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled TargetGroup", "TargetGroup", tg.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.TargetGroup{}).
		Complete(r)
}
