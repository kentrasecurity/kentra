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

	// Resolve targets from TargetPool (required)
	if enum.Spec.TargetPool == "" {
		return ctrl.Result{}, fmt.Errorf("targetPool is required")
	}

	resolvedTargets, err := resolver.ResolveTargetPool(ctx, enum.Spec.TargetPool, enum.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to resolve targetPool: %w", err)
	}

	if len(resolvedTargets) == 0 {
		return ctrl.Result{}, fmt.Errorf("no targets found in targetPool %s", enum.Spec.TargetPool)
	}

	// Get tool config to check for separators
	toolConfig, err := f.Configurator.GetToolConfig(enum.Spec.Tool)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get tool config: %w", err)
	}

	// Determine if we should batch targets
	batchTargets := toolConfig.EndpointSeparator != "" && toolConfig.PortSeparator != ""

	var jobSpecs []jobSpec
	if batchTargets {
		// Single job with all targets batched
		jobSpecs = []jobSpec{f.createBatchedJobSpec(resolvedTargets, toolConfig)}
	} else {
		// One job per target+port combination
		jobSpecs = f.createIndividualJobSpecs(resolvedTargets)
	}

	// Create jobs
	createdCount := 0
	for i, spec := range jobSpecs {
		jobName := fmt.Sprintf("%s-%d", enum.Name, i)

		attackSpec := &jobs.AttackSpec{
			Tool:          enum.Spec.Tool,
			Targets:       spec.Targets,
			Category:      enum.Spec.Category,
			Args:          enum.Spec.Args,
			HTTPProxy:     enum.Spec.HTTPProxy,
			AdditionalEnv: utils.ConvertEnvVars(enum.Spec.AdditionalEnv),
			Debug:         enum.Spec.Debug,
			Periodic:      enum.Spec.Periodic,
			Schedule:      enum.Spec.Schedule,
			Port:          spec.Port,
			Files:         files,
		}

		builder := &jobs.JobBuilder{
			Client:              f.Client,
			Scheme:              f.Scheme,
			Configurator:        f.Configurator,
			ControllerNamespace: f.ControllerNamespace,
			ResourceType:        "enumeration",
		}

		_, err := builder.ReconcileJob(ctx, enum, jobName, attackSpec, func(status *jobs.AttackStatus) {
			if i == 0 {
				enum.Status.State = status.State
				enum.Status.LastExecuted = status.LastExecuted
			}
		})

		if err != nil {
			log.Error(err, "Failed to create job", "jobName", jobName)
			continue
		}
		createdCount++
	}

	// Update overall status
	enum.Status.State = "Running"
	enum.Status.JobName = fmt.Sprintf("%d jobs", createdCount)
	enum.Status.ResolvedTarget = fmt.Sprintf("%d targets", len(resolvedTargets))

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

type jobSpec struct {
	Targets []string
	Port    string
}

func (f *EnumerationJobFactory) createBatchedJobSpec(targets []resolvers.ResolvedTarget, toolConfig *config.ToolConfig) jobSpec {
	// Group all endpoints
	endpoints := make([]string, 0, len(targets))
	portsMap := make(map[string]bool)

	for _, t := range targets {
		endpoints = append(endpoints, t.Endpoint)
		portsMap[t.Port] = true
	}

	// Remove duplicates
	endpoints = removeDuplicates(endpoints)

	// Collect unique ports
	var ports []string
	for port := range portsMap {
		ports = append(ports, port)
	}

	return jobSpec{
		Targets: endpoints,
		Port:    strings.Join(ports, toolConfig.PortSeparator),
	}
}

func (f *EnumerationJobFactory) createIndividualJobSpecs(targets []resolvers.ResolvedTarget) []jobSpec {
	specs := make([]jobSpec, len(targets))
	for i, t := range targets {
		specs[i] = jobSpec{
			Targets: []string{t.Endpoint},
			Port:    t.Port,
		}
	}
	return specs
}

func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func (r *EnumerationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Enumeration{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
