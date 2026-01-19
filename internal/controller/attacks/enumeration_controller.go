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

	// Get grouped targets by target name
	targetGroups, err := resolver.ResolveTargetPoolGrouped(ctx, enum.Spec.TargetPool, enum.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to resolve targetPool: %w", err)
	}

	if len(targetGroups) == 0 {
		return ctrl.Result{}, fmt.Errorf("no targets found in targetPool %s", enum.Spec.TargetPool)
	}

	// Get tool config to check for separators
	toolConfig, err := f.Configurator.GetToolConfig(enum.Spec.Tool)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get tool config: %w", err)
	}

	// Determine if we should batch targets
	hasEndpointSeparator := toolConfig.EndpointSeparator != ""
	hasPortSeparator := toolConfig.PortSeparator != ""

	// Create jobs based on grouping strategy
	createdCount := 0
	jobIndex := 0

	for _, group := range targetGroups {
		if hasEndpointSeparator && hasPortSeparator {
			// Batch all endpoints and ports into single job per target group
			jobName := fmt.Sprintf("%s-%s", enum.Name, group.Name)
			if err := f.createBatchedJob(ctx, enum, jobName, group, toolConfig, files); err != nil {
				log.Error(err, "Failed to create batched job", "target", group.Name)
				continue
			}
			createdCount++
		} else if hasEndpointSeparator && !hasPortSeparator {
			// Batch endpoints, separate jobs per port
			for portIdx, port := range group.Ports {
				jobName := fmt.Sprintf("%s-%s-port%d", enum.Name, group.Name, portIdx)
				singlePortGroup := resolvers.TargetGroup{
					Name:      group.Name,
					Endpoints: group.Endpoints,
					Ports:     []string{port},
				}
				if err := f.createBatchedJob(ctx, enum, jobName, singlePortGroup, toolConfig, files); err != nil {
					log.Error(err, "Failed to create job", "target", group.Name, "port", port)
					continue
				}
				createdCount++
			}
		} else if !hasEndpointSeparator && hasPortSeparator {
			// Batch ports, separate jobs per endpoint
			for endpointIdx, endpoint := range group.Endpoints {
				jobName := fmt.Sprintf("%s-%s-ep%d", enum.Name, group.Name, endpointIdx)
				singleEndpointGroup := resolvers.TargetGroup{
					Name:      group.Name,
					Endpoints: []string{endpoint},
					Ports:     group.Ports,
				}
				if err := f.createBatchedJob(ctx, enum, jobName, singleEndpointGroup, toolConfig, files); err != nil {
					log.Error(err, "Failed to create job", "target", group.Name, "endpoint", endpoint)
					continue
				}
				createdCount++
			}
		} else {
			// No separators: create individual job per endpoint+port combination
			for _, endpoint := range group.Endpoints {
				for _, port := range group.Ports {
					jobName := fmt.Sprintf("%s-%d", enum.Name, jobIndex)
					jobIndex++

					singleTargetGroup := resolvers.TargetGroup{
						Name:      group.Name,
						Endpoints: []string{endpoint},
						Ports:     []string{port},
					}
					if err := f.createBatchedJob(ctx, enum, jobName, singleTargetGroup, toolConfig, files); err != nil {
						log.Error(err, "Failed to create job", "endpoint", endpoint, "port", port)
						continue
					}
					createdCount++
				}
			}
		}
	}

	// Update overall status
	enum.Status.State = "Running"
	enum.Status.JobName = fmt.Sprintf("%d jobs", createdCount)
	enum.Status.ResolvedTarget = fmt.Sprintf("%d target groups", len(targetGroups))

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (f *EnumerationJobFactory) createBatchedJob(
	ctx context.Context,
	enum *securityv1alpha1.Enumeration,
	jobName string,
	group resolvers.TargetGroup,
	toolConfig *config.ToolConfig,
	files []string,
) error {
	// Determine separators
	endpointSep := toolConfig.EndpointSeparator
	if endpointSep == "" {
		endpointSep = " "
	}

	portSep := toolConfig.PortSeparator
	if portSep == "" {
		portSep = ","
	}

	// Join endpoints and ports with separators
	targetString := strings.Join(group.Endpoints, endpointSep)
	portString := strings.Join(group.Ports, portSep)

	attackSpec := &jobs.AttackSpec{
		Tool:          enum.Spec.Tool,
		Targets:       []string{targetString},
		Category:      enum.Spec.Category,
		Args:          enum.Spec.Args,
		HTTPProxy:     enum.Spec.HTTPProxy,
		AdditionalEnv: utils.ConvertEnvVars(enum.Spec.AdditionalEnv),
		Debug:         enum.Spec.Debug,
		Periodic:      enum.Spec.Periodic,
		Schedule:      enum.Spec.Schedule,
		Port:          portString,
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
		enum.Status.State = status.State
		enum.Status.LastExecuted = status.LastExecuted
	})

	return err
}

func (r *EnumerationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Enumeration{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
