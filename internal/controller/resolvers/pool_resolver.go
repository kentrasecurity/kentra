package resolvers

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// PoolResolver resolves pool references to actual values
type PoolResolver struct {
	client client.Client
}

// New creates a new PoolResolver
func New(c client.Client) *PoolResolver {
	return &PoolResolver{client: c}
}

// ResolveStoragePool resolves a StoragePool reference to file list
func (r *PoolResolver) ResolveStoragePool(ctx context.Context, poolName, namespace string) ([]string, error) {
	if poolName == "" {
		return []string{}, nil
	}
	pool := &securityv1alpha1.StoragePool{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: poolName, Namespace: namespace}, pool); err != nil {
		return nil, err
	}
	return pool.Spec.Files, nil
}

// ResolveTargetPool resolves a TargetPool reference to target list and port
func (r *PoolResolver) ResolveTargetPool(ctx context.Context, poolName, namespace string) ([]string, string, error) {
	if poolName == "" {
		return []string{}, "", nil
	}
	pool := &securityv1alpha1.TargetPool{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: poolName, Namespace: namespace}, pool); err != nil {
		return nil, "", err
	}
	return pool.GetAllTargets(), pool.Spec.Port, nil
}

// ResolveTargetWithPort resolves target and port from TargetPool
func (r *PoolResolver) ResolveTargetWithPort(ctx context.Context, poolName, directTarget, directPort, namespace string) ([]string, string) {
	if poolName != "" {
		targets, port, err := r.ResolveTargetPool(ctx, poolName, namespace)
		if err == nil && len(targets) > 0 {
			return targets, port
		}
	}

	// No pool or pool failed - return empty
	return []string{}, directPort
}

// ResolveAssetPool resolves an AssetPool reference
func (r *PoolResolver) ResolveAssetPool(ctx context.Context, poolName, namespace string) ([]securityv1alpha1.AssetItem, error) {
	if poolName == "" {
		return []securityv1alpha1.AssetItem{}, nil
	}
	pool := &securityv1alpha1.AssetPool{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: poolName, Namespace: namespace}, pool); err != nil {
		return nil, err
	}
	return pool.GetAllAssets(), nil
}
