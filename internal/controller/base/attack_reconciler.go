package base

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kentrasecurity/kentra/internal/controller/config"
)

// AttackResource represents any attack-type Kubernetes resource
type AttackResource interface {
	client.Object
	GetGeneration() int64
}

// BaseAttackReconciler handles common reconciliation logic for all attacks
type BaseAttackReconciler struct {
	Client              client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
	ResourceType        string
}

// ReconcileAttack implements the common reconciliation pattern
func (r *BaseAttackReconciler) ReconcileAttack(
	ctx context.Context,
	req ctrl.Request,
	resource AttackResource,
	jobFactory JobFactory,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. Load tool configuration
	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool configurations, retrying...")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// 2. Fetch resource
	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 3. Validate namespace
	if err := r.validateNamespace(ctx, resource.GetNamespace()); err != nil {
		return ctrl.Result{}, err
	}

	// 4. Ensure labels
	if err := r.ensureLabels(ctx, resource); err != nil {
		return ctrl.Result{}, err
	}

	// 5. Create/update jobs using the factory
	return jobFactory.ReconcileJobs(ctx, resource)
}

func (r *BaseAttackReconciler) validateNamespace(ctx context.Context, namespace string) error {
	return ValidateNamespace(ctx, r.Client, namespace)
}

func (r *BaseAttackReconciler) ensureLabels(ctx context.Context, resource AttackResource) error {
	labels := resource.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	if labels["kentra.sh/resource-type"] != "attack" {
		labels["kentra.sh/resource-type"] = "attack"
		resource.SetLabels(labels)
		return r.Client.Update(ctx, resource)
	}

	return nil
}
