package attacks

import (
	"context"
	"fmt"
	"strings"
	"time"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	"github.com/kentrasecurity/kentra/internal/controller/base"
	"github.com/kentrasecurity/kentra/internal/controller/config"
	"github.com/kentrasecurity/kentra/internal/controller/jobs"
	"github.com/kentrasecurity/kentra/internal/controller/resolvers"
	"github.com/kentrasecurity/kentra/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EnumerationReconciler reconciles Enumeration resources
type EnumerationReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

//+kubebuilder:rbac:groups=kentra.sh,resources=enumerations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=enumerations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools;storagepools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets;namespaces,verbs=get;list;watch;create

func (r *EnumerationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	baseReconciler := &base.BaseAttackReconciler{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
		ResourceType:        "enumeration",
	}
	enum := &securityv1alpha1.Enumeration{}
	factory := &EnumerationJobFactory{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
	}
	return baseReconciler.ReconcileAttack(ctx, req, enum, factory)
}

// EnumerationJobFactory creates jobs for Enumeration resources
type EnumerationJobFactory struct {
	Client              client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

func (f *EnumerationJobFactory) ReconcileJobs(ctx context.Context, resource base.AttackResource) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	enum := resource.(*securityv1alpha1.Enumeration)

	// Resolve pools
	resolver := resolvers.New(f.Client)
	files, _ := resolver.ResolveStoragePool(ctx, enum.Spec.StoragePool, enum.Namespace)

	// Resolve targets from pool or use direct targets
	var targets []string
	var port string
	if enum.Spec.TargetPool != "" {
		var directTarget string
		if len(enum.Spec.Targets) > 0 {
			directTarget = enum.Spec.Targets[0]
		}
		targets, port = resolver.ResolveTargetWithPort(ctx, enum.Spec.TargetPool, directTarget, enum.Spec.Port, enum.Namespace)
	} else {
		targets = enum.Spec.Targets
		port = enum.Spec.Port
	}

	if len(targets) == 0 {
		return ctrl.Result{}, fmt.Errorf("no targets specified")
	}

	// Store resolved targets in status
	enum.Status.ResolvedTarget = strings.Join(targets, ",")
	enum.Status.ResolvedPort = port

	// Create one job per target
	createdCount := 0
	for i, target := range targets {
		jobName := fmt.Sprintf("%s-%d", enum.Name, i)

		spec := &jobs.AttackSpec{
			Tool:          enum.Spec.Tool,
			Targets:       []string{target}, // Single target per job
			Category:      enum.Spec.Category,
			Args:          enum.Spec.Args,
			HTTPProxy:     enum.Spec.HTTPProxy,
			AdditionalEnv: utils.ConvertEnvVars(enum.Spec.AdditionalEnv),
			Debug:         enum.Spec.Debug,
			Periodic:      enum.Spec.Periodic,
			Schedule:      enum.Spec.Schedule,
			Port:          port,
			Files:         files,
		}

		builder := &jobs.JobBuilder{
			Client:              f.Client,
			Scheme:              f.Scheme,
			Configurator:        f.Configurator,
			ControllerNamespace: f.ControllerNamespace,
			ResourceType:        "enumeration",
		}

		_, err := builder.ReconcileJob(ctx, enum, jobName, spec, func(status *jobs.AttackStatus) {
			// Update status for the first job only to avoid conflicts
			if i == 0 {
				enum.Status.State = status.State
				enum.Status.LastExecuted = status.LastExecuted
			}
		})

		if err != nil {
			log.Error(err, "Failed to create job for target", "target", target)
			continue
		}
		createdCount++
	}

	// Update overall status
	enum.Status.State = "Running"
	enum.Status.JobName = fmt.Sprintf("%d jobs", createdCount)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *EnumerationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Enumeration{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
