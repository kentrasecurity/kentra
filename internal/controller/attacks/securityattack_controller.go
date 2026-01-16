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

type SecurityAttackReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

//+kubebuilder:rbac:groups=kentra.sh,resources=securityattacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=securityattacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets;namespaces,verbs=get;list;watch;create

func (r *SecurityAttackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	baseReconciler := &base.BaseAttackReconciler{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
		ResourceType:        "security-attack",
	}

	securityAttack := &securityv1alpha1.SecurityAttack{}
	factory := &SecurityAttackJobFactory{
		Client:              r.Client,
		Scheme:              r.Scheme,
		Configurator:        r.Configurator,
		ControllerNamespace: r.ControllerNamespace,
	}

	return baseReconciler.ReconcileAttack(ctx, req, securityAttack, factory)
}

type SecurityAttackJobFactory struct {
	Client              client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
}

func (f *SecurityAttackJobFactory) ReconcileJobs(ctx context.Context, resource base.AttackResource) (ctrl.Result, error) {
	sa := resource.(*securityv1alpha1.SecurityAttack)
	resolver := resolvers.New(f.Client)

	// Resolve targets from pool or use direct targets
	var targets []string
	var err error
	if sa.Spec.TargetPool != "" {
		// Get targets from pool
		var directTarget string
		if sa.Spec.Target != "" {
			directTarget = sa.Spec.Target
		} else if len(sa.Spec.Targets) > 0 {
			directTarget = sa.Spec.Targets[0]
		}
		targets, err = resolver.ResolveTarget(ctx, sa.Spec.TargetPool, directTarget, sa.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Use direct targets (with fallback to deprecated Target field)
		if len(sa.Spec.Targets) > 0 {
			targets = sa.Spec.Targets
		} else if sa.Spec.Target != "" {
			targets = []string{sa.Spec.Target}
		}
	}

	if len(targets) == 0 {
		return ctrl.Result{}, fmt.Errorf("neither target nor targetPool specified")
	}

	sa.Status.ResolvedTarget = strings.Join(targets, ",")

	spec := &jobs.AttackSpec{
		Tool:          sa.Spec.Tool,
		Targets:       targets,
		Category:      sa.Spec.Category,
		Args:          sa.Spec.Args,
		HTTPProxy:     sa.Spec.HTTPProxy,
		AdditionalEnv: utils.ConvertEnvVars(sa.Spec.AdditionalEnv),
		Debug:         sa.Spec.Debug,
		Periodic:      sa.Spec.Periodic,
		Schedule:      sa.Spec.Schedule,
		Files:         []string{},
	}

	builder := &jobs.JobBuilder{
		Client:              f.Client,
		Scheme:              f.Scheme,
		Configurator:        f.Configurator,
		ControllerNamespace: f.ControllerNamespace,
		ResourceType:        "security-attack",
	}

	return builder.ReconcileJob(ctx, sa, sa.Name, spec, func(status *jobs.AttackStatus) {
		sa.Status.State = status.State
		sa.Status.LastExecuted = status.LastExecuted
	})
}

func (r *SecurityAttackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.SecurityAttack{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
