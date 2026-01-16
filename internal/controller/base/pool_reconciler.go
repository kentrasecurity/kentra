package base

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PoolResource represents any pool-type Kubernetes resource
type PoolResource interface {
	client.Object
	GetGeneration() int64
}

// PoolStatusUpdater updates pool-specific status fields
type PoolStatusUpdater interface {
	UpdateStatus(ctx context.Context, resource PoolResource) error
}

// BasePoolReconciler handles common reconciliation logic for all pools
type BasePoolReconciler struct {
	Client       client.Client
	Scheme       *runtime.Scheme
	ResourceType string // "asset", "storage", "target"
}

// ReconcilePool implements the common reconciliation pattern for pools
func (r *BasePoolReconciler) ReconcilePool(
	ctx context.Context,
	req ctrl.Request,
	resource PoolResource,
	statusUpdater PoolStatusUpdater,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. Fetch resource
	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Pool resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get pool resource")
		return ctrl.Result{}, err
	}

	// 2. Validate namespace
	if err := ValidateNamespace(ctx, r.Client, resource.GetNamespace()); err != nil {
		log.Error(err, "Namespace not managed by Kentra", "namespace", resource.GetNamespace())
		return ctrl.Result{}, err
	}

	// 3. Ensure labels
	if err := r.ensureLabels(ctx, resource); err != nil {
		return ctrl.Result{}, err
	}

	// 4. Update pool-specific status
	if err := statusUpdater.UpdateStatus(ctx, resource); err != nil {
		log.Error(err, "Failed to update pool status")
		return ctrl.Result{}, err
	}

	// 5. Update status writer with timestamp
	if err := r.updateTimestamp(ctx, resource); err != nil {
		log.Error(err, "Failed to update timestamp")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled pool", "type", r.ResourceType, "name", resource.GetName())
	return ctrl.Result{}, nil
}

func (r *BasePoolReconciler) ensureLabels(ctx context.Context, resource PoolResource) error {
	labels := resource.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	expectedLabel := r.ResourceType
	if labels["kentra.sh/resource-type"] != expectedLabel {
		labels["kentra.sh/resource-type"] = expectedLabel
		resource.SetLabels(labels)
		if err := r.Client.Update(ctx, resource); err != nil {
			return err
		}
		// Return with requeue to ensure status update happens after label update
		return fmt.Errorf("labels updated, requeuing")
	}

	return nil
}

func (r *BasePoolReconciler) updateTimestamp(ctx context.Context, resource PoolResource) error {
	// This is a generic approach - each pool controller will implement
	// their specific status update through the PoolStatusUpdater interface
	return r.Client.Status().Update(ctx, resource)
}
