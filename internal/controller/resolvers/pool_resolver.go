package resolvers

import (
	"context"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// ResolveTarget resolves a TargetPool or returns direct target
func (r *PoolResolver) ResolveTarget(ctx context.Context, poolName, directTarget, namespace string) ([]string, error) {
	if poolName == "" {
		if directTarget == "" {
			return []string{}, nil
		}
		return []string{directTarget}, nil
	}
	pool := &securityv1alpha1.TargetPool{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: poolName, Namespace: namespace}, pool); err != nil {
		return nil, err
	}
	return pool.Spec.Targets, nil
}

// ResolveTargetWithPort resolves target and port from TargetPool
func (r *PoolResolver) ResolveTargetWithPort(ctx context.Context, poolName, directTarget, directPort, namespace string) ([]string, string) {
	if poolName == "" {
		if directTarget == "" {
			return []string{}, directPort
		}
		return []string{directTarget}, directPort
	}
	pool := &securityv1alpha1.TargetPool{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: poolName, Namespace: namespace}, pool); err != nil {
		if directTarget == "" {
			return []string{}, directPort
		}
		return []string{directTarget}, directPort
	}
	port := directPort
	if pool.Spec.Port != "" {
		port = pool.Spec.Port
	}
	return pool.Spec.Targets, port
}

// ResolveAssetPool resolves an AssetPool reference
func (r *PoolResolver) ResolveAssetPool(ctx context.Context, poolName, namespace string) ([]securityv1alpha1.AssetPoolItem, error) {
	if poolName == "" {
		return []securityv1alpha1.AssetPoolItem{}, nil
	}
	pool := &securityv1alpha1.AssetPool{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: poolName, Namespace: namespace}, pool); err != nil {
		return nil, err
	}
	return pool.Spec.Pool, nil
}
