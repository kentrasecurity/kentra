package attacks

import (
	"context"
	"fmt"
	"strings"
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

	// AssetPool mode: multiple jobs
	if osint.Spec.AssetPool != "" {
		return f.handleAssetPoolMode(ctx, osint, resolver, files)
	}

	// Target mode: single job
	return f.handleTargetMode(ctx, osint, resolver, files)
}

func (f *OsintJobFactory) handleTargetMode(
	ctx context.Context,
	osint *securityv1alpha1.Osint,
	resolver *resolvers.PoolResolver,
	files []string,
) (ctrl.Result, error) {
	// Resolve targets from pool or use direct targets
	var targets []string
	if osint.Spec.TargetPool != "" {
		// Get targets from pool
		var directTarget string
		if osint.Spec.Target != "" {
			directTarget = osint.Spec.Target
		} else if len(osint.Spec.Targets) > 0 {
			directTarget = osint.Spec.Targets[0]
		}
		var err error
		targets, err = resolver.ResolveTarget(ctx, osint.Spec.TargetPool, directTarget, osint.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Use direct targets (with fallback to deprecated Target field)
		if len(osint.Spec.Targets) > 0 {
			targets = osint.Spec.Targets
		} else if osint.Spec.Target != "" {
			targets = []string{osint.Spec.Target}
		}
	}

	if len(targets) == 0 {
		return ctrl.Result{}, fmt.Errorf("no target specified")
	}

	osint.Status.ResolvedTarget = strings.Join(targets, ",")
	return f.createJob(ctx, osint, osint.Name, targets, files, nil)
}

func (f *OsintJobFactory) handleAssetPoolMode(
	ctx context.Context,
	osint *securityv1alpha1.Osint,
	resolver *resolvers.PoolResolver,
	files []string,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	assetPoolItems, err := resolver.ResolveAssetPool(ctx, osint.Spec.AssetPool, osint.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to resolve AssetPool: %w", err)
	}
	if len(assetPoolItems) == 0 {
		return ctrl.Result{}, fmt.Errorf("assetPool %s has no items", osint.Spec.AssetPool)
	}

	// Get targets for asset pool mode
	var targets []string
	if osint.Spec.Target != "" {
		targets = []string{osint.Spec.Target}
	} else if len(osint.Spec.Targets) > 0 {
		targets = osint.Spec.Targets
	}

	// Create one job per group
	createdCount := 0
	for i, item := range assetPoolItems {
		if len(item.Assets) == 0 {
			continue
		}

		jobName := utils.GenerateJobName(osint.Name, item.Name, i)
		_, err := f.createJob(ctx, osint, jobName, targets, files, item.Assets)
		if err != nil {
			log.Error(err, "Failed to create job for group", "group", item.Name)
			continue
		}
		createdCount++
	}

	// Update status
	osint.Status.State = "Running"
	osint.Status.JobName = fmt.Sprintf("%d jobs", createdCount)
	osint.Status.LastExecuted = time.Now().Format(time.RFC3339)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
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
		Port:          osint.Spec.Port,
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
