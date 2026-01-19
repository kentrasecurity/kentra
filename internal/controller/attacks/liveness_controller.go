/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

type LivenessReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

//+kubebuilder:rbac:groups=kentra.sh,resources=livenesses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=livenesses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets;namespaces,verbs=get;list;watch;create

func (r *LivenessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	baseReconciler := &base.BaseAttackReconciler{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
		ResourceType:        "liveness",
	}

	liveness := &securityv1alpha1.Liveness{}
	factory := &LivenessJobFactory{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
	}

	return baseReconciler.ReconcileAttack(ctx, req, liveness, factory)
}

type LivenessJobFactory struct {
	Client              client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

func (f *LivenessJobFactory) ReconcileJobs(ctx context.Context, resource base.AttackResource) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	liveness := resource.(*securityv1alpha1.Liveness)
	resolver := resolvers.New(f.Client)

	// Resolve targets from TargetPool (required)
	if liveness.Spec.TargetPool == "" {
		return ctrl.Result{}, fmt.Errorf("targetPool is required")
	}

	// Get grouped targets by target name
	targetGroups, err := resolver.ResolveTargetPoolGrouped(ctx, liveness.Spec.TargetPool, liveness.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to resolve targetPool: %w", err)
	}

	if len(targetGroups) == 0 {
		return ctrl.Result{}, fmt.Errorf("no targets found in targetPool %s", liveness.Spec.TargetPool)
	}

	// Get tool config to check for separators
	toolConfig, err := f.Configurator.GetToolConfig(liveness.Spec.Tool)
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
			jobName := fmt.Sprintf("%s-%s", liveness.Name, group.Name)
			if err := f.createBatchedJob(ctx, liveness, jobName, group, toolConfig); err != nil {
				log.Error(err, "Failed to create batched job", "target", group.Name)
				continue
			}
			createdCount++
		} else if hasEndpointSeparator && !hasPortSeparator {
			// Batch endpoints, separate jobs per port
			for portIdx, port := range group.Ports {
				jobName := fmt.Sprintf("%s-%s-port%d", liveness.Name, group.Name, portIdx)
				singlePortGroup := resolvers.TargetGroup{
					Name:      group.Name,
					Endpoints: group.Endpoints,
					Ports:     []string{port},
				}
				if err := f.createBatchedJob(ctx, liveness, jobName, singlePortGroup, toolConfig); err != nil {
					log.Error(err, "Failed to create job", "target", group.Name, "port", port)
					continue
				}
				createdCount++
			}
		} else if !hasEndpointSeparator && hasPortSeparator {
			// Batch ports, separate jobs per endpoint
			for endpointIdx, endpoint := range group.Endpoints {
				jobName := fmt.Sprintf("%s-%s-ep%d", liveness.Name, group.Name, endpointIdx)
				singleEndpointGroup := resolvers.TargetGroup{
					Name:      group.Name,
					Endpoints: []string{endpoint},
					Ports:     group.Ports,
				}
				if err := f.createBatchedJob(ctx, liveness, jobName, singleEndpointGroup, toolConfig); err != nil {
					log.Error(err, "Failed to create job", "target", group.Name, "endpoint", endpoint)
					continue
				}
				createdCount++
			}
		} else {
			// No separators: create individual job per endpoint+port combination
			for _, endpoint := range group.Endpoints {
				for _, port := range group.Ports {
					jobName := fmt.Sprintf("%s-%d", liveness.Name, jobIndex)
					jobIndex++

					singleTargetGroup := resolvers.TargetGroup{
						Name:      group.Name,
						Endpoints: []string{endpoint},
						Ports:     []string{port},
					}
					if err := f.createBatchedJob(ctx, liveness, jobName, singleTargetGroup, toolConfig); err != nil {
						log.Error(err, "Failed to create job", "endpoint", endpoint, "port", port)
						continue
					}
					createdCount++
				}
			}
		}
	}

	// Update overall status
	liveness.Status.State = "Running"
	liveness.Status.JobName = fmt.Sprintf("%d jobs", createdCount)
	liveness.Status.ResolvedTarget = fmt.Sprintf("%d target groups", len(targetGroups))

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (f *LivenessJobFactory) createBatchedJob(
	ctx context.Context,
	liveness *securityv1alpha1.Liveness,
	jobName string,
	group resolvers.TargetGroup,
	toolConfig *config.ToolConfig,
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

	spec := &jobs.AttackSpec{
		Tool:          liveness.Spec.Tool,
		Targets:       []string{targetString},
		Category:      liveness.Spec.Category,
		Args:          liveness.Spec.Args,
		HTTPProxy:     liveness.Spec.HTTPProxy,
		AdditionalEnv: utils.ConvertEnvVars(liveness.Spec.AdditionalEnv),
		Debug:         liveness.Spec.Debug,
		Periodic:      liveness.Spec.Periodic,
		Schedule:      liveness.Spec.Schedule,
		Port:          portString,
		Files:         []string{},
	}

	builder := &jobs.JobBuilder{
		Client:              f.Client,
		Scheme:              f.Scheme,
		Configurator:        f.Configurator,
		ControllerNamespace: f.ControllerNamespace,
		ResourceType:        "liveness",
	}

	_, err := builder.ReconcileJob(ctx, liveness, jobName, spec, func(status *jobs.AttackStatus) {
		liveness.Status.State = status.State
		liveness.Status.LastExecuted = status.LastExecuted
	})

	return err
}

func (r *LivenessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Liveness{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
