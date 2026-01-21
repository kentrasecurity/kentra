package base

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

// JobFactory creates and manages jobs for attack resources
type JobFactory interface {
	ReconcileJobs(ctx context.Context, resource AttackResource) (ctrl.Result, error)
}
