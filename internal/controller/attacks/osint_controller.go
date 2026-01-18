package attacks

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	"github.com/kentrasecurity/kentra/internal/controller/base"
	"github.com/kentrasecurity/kentra/internal/controller/config"
	"github.com/kentrasecurity/kentra/internal/controller/jobs"
	"github.com/kentrasecurity/kentra/internal/controller/resolvers"
	"github.com/kentrasecurity/kentra/internal/controller/utils"
)

type OsintReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

//+kubebuilder:rbac:groups=kentra.sh,resources=osints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=osints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools;assetpools;storagepools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets;namespaces,verbs=get;list;watch;create

func (r *OsintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	baseReconciler := &base.BaseAttackReconciler{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
		ResourceType:        "osint",
	}

	osint := &securityv1alpha1.Osint{}
	factory := &OsintJobFactory{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
	}

	return baseReconciler.ReconcileAttack(ctx, req, osint, factory)
}

type OsintJobFactory struct {
	Client              client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

func (f *OsintJobFactory) ReconcileJobs(ctx context.Context, resource base.AttackResource) (ctrl.Result, error) {
	osint := resource.(*securityv1alpha1.Osint)
	resolver := resolvers.New(f.Client)

	// Resolve storage files
	files, _ := resolver.ResolveStoragePool(ctx, osint.Spec.StoragePool, osint.Namespace)

	return f.handleAssetPoolMode(ctx, osint, resolver, files)
}

func (f *OsintJobFactory) handleAssetPoolMode(
	ctx context.Context,
	osint *securityv1alpha1.Osint,
	resolver *resolvers.PoolResolver,
	files []string,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Resolve AssetPool
	assetItems, err := resolver.ResolveAssetPool(ctx, osint.Spec.AssetPool, osint.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to resolve AssetPool: %w", err)
	}
	if len(assetItems) == 0 {
		return ctrl.Result{}, fmt.Errorf("assetPool %s has no items", osint.Spec.AssetPool)
	}

	// Get tool config to check for separator
	toolConfig, err := f.Configurator.GetToolConfig(osint.Spec.Tool)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get tool config: %w", err)
	}

	// Get required asset types from the tool's command template
	requiredAssetTypes, err := f.Configurator.GetRequiredAssetTypes(osint.Spec.Tool)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get required asset types for tool %s: %w", osint.Spec.Tool, err)
	}

	// Filter assets based on what the template requires
	filteredAssets := filterAssetsByTypes(assetItems, requiredAssetTypes)
	if len(filteredAssets) == 0 {
		return ctrl.Result{}, fmt.Errorf("no assets of required types %v found in assetPool for tool %s",
			requiredAssetTypes, osint.Spec.Tool)
	}

	log.Info("Filtered assets for tool",
		"tool", osint.Spec.Tool,
		"requiredTypes", requiredAssetTypes,
		"totalAssets", len(assetItems),
		"filteredAssets", len(filteredAssets))

	// Determine if we should batch assets
	batchAssets := toolConfig.Separator != ""

	var createdCount int
	if batchAssets {
		// Create single job with all assets batched
		log.Info("Batching assets with separator", "separator", toolConfig.Separator, "assetCount", len(filteredAssets))
		createdCount, err = f.createBatchedJob(ctx, osint, filteredAssets, files)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Create one job per asset
		log.Info("Creating individual jobs per asset", "assetCount", len(filteredAssets))
		createdCount, err = f.createIndividualJobs(ctx, osint, filteredAssets, files)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status
	osint.Status.State = "Running"
	osint.Status.JobName = fmt.Sprintf("%d jobs", createdCount)
	osint.Status.LastExecuted = time.Now().Format(time.RFC3339)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// createBatchedJob creates a single job with all assets
func (f *OsintJobFactory) createBatchedJob(
	ctx context.Context,
	osint *securityv1alpha1.Osint,
	assets []securityv1alpha1.AssetItem,
	files []string,
) (int, error) {
	log := log.FromContext(ctx)

	jobName := fmt.Sprintf("%s-batch", osint.Name)

	_, err := f.createJob(ctx, osint, jobName, []string{}, files, assets)
	if err != nil {
		log.Error(err, "Failed to create batched job")
		return 0, err
	}

	return 1, nil
}

// createIndividualJobs creates one job per asset
func (f *OsintJobFactory) createIndividualJobs(
	ctx context.Context,
	osint *securityv1alpha1.Osint,
	assets []securityv1alpha1.AssetItem,
	files []string,
) (int, error) {
	log := log.FromContext(ctx)
	createdCount := 0

	for i, asset := range assets {
		jobName := fmt.Sprintf("%s-%d", osint.Name, i)

		_, err := f.createJob(ctx, osint, jobName, []string{}, files, []securityv1alpha1.AssetItem{asset})
		if err != nil {
			log.Error(err, "Failed to create job for asset", "type", asset.Type, "value", asset.Value)
			continue
		}
		createdCount++
	}

	return createdCount, nil
}

// filterAssetsByTypes filters assets to only include specified types
func filterAssetsByTypes(assets []securityv1alpha1.AssetItem, types []string) []securityv1alpha1.AssetItem {
	if len(types) == 0 {
		// If no types specified, include all
		return assets
	}

	// Create a map for quick lookup
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	// Filter assets
	var filtered []securityv1alpha1.AssetItem
	for _, asset := range assets {
		if typeMap[asset.Type] {
			filtered = append(filtered, asset)
		}
	}

	return filtered
}

func (f *OsintJobFactory) createJob(
	ctx context.Context,
	osint *securityv1alpha1.Osint,
	jobName string,
	targets []string,
	files []string,
	assets []securityv1alpha1.AssetItem,
) (ctrl.Result, error) {
	spec := &jobs.AttackSpec{
		Tool:          osint.Spec.Tool,
		Targets:       targets,
		Category:      osint.Spec.Category,
		Args:          osint.Spec.Args,
		HTTPProxy:     osint.Spec.HTTPProxy,
		AdditionalEnv: utils.ConvertEnvVars(osint.Spec.AdditionalEnv),
		Debug:         osint.Spec.Debug,
		Periodic:      osint.Spec.Periodic,
		Schedule:      osint.Spec.Schedule,
		Port:          "",
		Files:         files,
		Assets:        assets,
	}

	builder := &jobs.JobBuilder{
		Client:              f.Client,
		Scheme:              f.Scheme,
		Configurator:        f.Configurator,
		ControllerNamespace: f.ControllerNamespace,
		ResourceType:        "osint",
	}

	return builder.ReconcileJob(ctx, osint, jobName, spec, func(status *jobs.AttackStatus) {
		osint.Status.State = status.State
		osint.Status.LastExecuted = status.LastExecuted
		osint.Status.JobName = status.JobName
	})
}

func (r *OsintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Osint{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
