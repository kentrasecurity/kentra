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
	liveness := resource.(*securityv1alpha1.Liveness)
	resolver := resolvers.New(f.Client)

	// Resolve targets from pool or use direct targets
	var targets []string
	var err error
	if liveness.Spec.TargetPool != "" {
		// Get targets from pool
		var directTarget string
		if liveness.Spec.Target != "" {
			directTarget = liveness.Spec.Target
		} else if len(liveness.Spec.Targets) > 0 {
			directTarget = liveness.Spec.Targets[0]
		}
		targets, err = resolver.ResolveTarget(ctx, liveness.Spec.TargetPool, directTarget, liveness.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Use direct targets (with fallback to deprecated Target field)
		if len(liveness.Spec.Targets) > 0 {
			targets = liveness.Spec.Targets
		} else if liveness.Spec.Target != "" {
			targets = []string{liveness.Spec.Target}
		}
	}

	if len(targets) == 0 {
		return ctrl.Result{}, fmt.Errorf("neither target nor targetPool specified")
	}

	liveness.Status.ResolvedTarget = strings.Join(targets, ",")

	// Generate unique job name for one-time jobs
	jobName := liveness.Name
	if !liveness.Spec.Periodic {
		jobName = fmt.Sprintf("%s-job-%d", liveness.Name, time.Now().Unix())
	}

	spec := &jobs.AttackSpec{
		Tool:          liveness.Spec.Tool,
		Targets:       targets,
		Category:      liveness.Spec.Category,
		Args:          liveness.Spec.Args,
		HTTPProxy:     liveness.Spec.HTTPProxy,
		AdditionalEnv: utils.ConvertEnvVars(liveness.Spec.AdditionalEnv),
		Debug:         liveness.Spec.Debug,
		Periodic:      liveness.Spec.Periodic,
		Schedule:      liveness.Spec.Schedule,
		Files:         []string{},
	}

	builder := &jobs.JobBuilder{
		Client:              f.Client,
		Scheme:              f.Scheme,
		Configurator:        f.Configurator,
		ControllerNamespace: f.ControllerNamespace,
		ResourceType:        "liveness",
	}

	return builder.ReconcileJob(ctx, liveness, jobName, spec, func(status *jobs.AttackStatus) {
		liveness.Status.State = status.State
		liveness.Status.LastExecuted = status.LastExecuted
	})
}

func (r *LivenessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Liveness{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
