package resolvers

import (
	"context"
	"fmt"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	"github.com/kentrasecurity/kentra/internal/controller/utils"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PoolResolver struct {
	client client.Client
}

func New(c client.Client) *PoolResolver {
	return &PoolResolver{client: c}
}

// ResolvedTarget represents a single endpoint+port combination
type ResolvedTarget struct {
	Endpoint string
	Port     string
}

// ResolveTargetPool resolves a TargetPool and returns expanded targets
// Returns: targets (endpoints), ports (comma-separated if multiple), or error
func (r *PoolResolver) ResolveTargetPool(ctx context.Context, poolName, namespace string) ([]ResolvedTarget, error) {
	if poolName == "" {
		return nil, fmt.Errorf("targetPool name cannot be empty")
	}

	pool := &securityv1alpha1.TargetPool{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Name:      poolName,
		Namespace: namespace,
	}, pool); err != nil {
		return nil, fmt.Errorf("failed to get TargetPool: %w", err)
	}

	if len(pool.Spec.Targets) == 0 {
		return nil, fmt.Errorf("targetPool %s has no targets", poolName)
	}

	var allTargets []ResolvedTarget

	for _, target := range pool.Spec.Targets {
		// Expand endpoints (CIDRs to IPs)
		expandedEndpoints, err := utils.ExpandEndpoints(target.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to expand endpoints for target %s: %w", target.Name, err)
		}

		// Expand ports (ranges to individual ports)
		expandedPorts, err := utils.ExpandPorts(target.Port)
		if err != nil {
			return nil, fmt.Errorf("failed to expand ports for target %s: %w", target.Name, err)
		}

		// Create all combinations of endpoint+port
		for _, endpoint := range expandedEndpoints {
			for _, port := range expandedPorts {
				allTargets = append(allTargets, ResolvedTarget{
					Endpoint: endpoint,
					Port:     port,
				})
			}
		}
	}

	return allTargets, nil
}

// ResolveAssetPool resolves an AssetPool and returns all assets
// This dynamically handles any asset type defined by the user
func (r *PoolResolver) ResolveAssetPool(ctx context.Context, poolName, namespace string) ([]securityv1alpha1.AssetItem, error) {
	if poolName == "" {
		return nil, fmt.Errorf("assetPool name cannot be empty")
	}

	pool := &securityv1alpha1.AssetPool{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Name:      poolName,
		Namespace: namespace,
	}, pool); err != nil {
		return nil, fmt.Errorf("failed to get AssetPool: %w", err)
	}

	var assets []securityv1alpha1.AssetItem

	// Iterate over all asset types dynamically
	for assetType, values := range pool.Spec.Asset {
		for _, value := range values {
			if value != "" { // Skip empty values
				assets = append(assets, securityv1alpha1.AssetItem{
					Type:  assetType,
					Value: value,
				})
			}
		}
	}

	return assets, nil
}

// ResolveStoragePool resolves a StoragePool and returns file list
func (r *PoolResolver) ResolveStoragePool(ctx context.Context, poolName, namespace string) ([]string, error) {
	if poolName == "" {
		return []string{}, nil
	}

	pool := &securityv1alpha1.StoragePool{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Name:      poolName,
		Namespace: namespace,
	}, pool); err != nil {
		return nil, fmt.Errorf("failed to get StoragePool: %w", err)
	}

	return pool.Spec.Files, nil
}
