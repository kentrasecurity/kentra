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

package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kttack/kttack/api/v1alpha1"
)

// LivenessReconciler reconciles a Liveness object
type LivenessReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Configurator *ToolsConfigurator
}

//+kubebuilder:rbac:groups=kttack.io,resources=livenesses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=livenesses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=livenesses/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps,verbs=get;list;watch

// Reconcile implements reconciliation for Liveness resources
func (r *LivenessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Load tool configurations from ConfigMap if not already loaded
	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool specifications ConfigMap - controller cannot proceed", "ConfigMap", "tool-specs")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Fetch the Liveness resource
	liveness := &securityv1alpha1.Liveness{}
	if err := r.Get(ctx, req.NamespacedName, liveness); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Liveness resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Liveness")
		return ctrl.Result{}, err
	}

	// Determine target namespace
	targetNamespace := liveness.Namespace

	// Generate names for Job/CronJob
	baseName := fmt.Sprintf("live-%s", liveness.Name)
	jobName := fmt.Sprintf("%s-job-%d", baseName, time.Now().Unix())
	cronJobName := fmt.Sprintf("%s-cronjob", baseName)

	// Convert to SecurityResource adapter
	adapter := &livenessAdapter{liveness}

	// Check if we need to create a job or cronjob
	if liveness.Spec.Periodic {
		// Create or update CronJob
		cronJob := &batchv1.CronJob{}
		cronJobNN := types.NamespacedName{Name: cronJobName, Namespace: targetNamespace}

		err := r.Get(ctx, cronJobNN, cronJob)
		if err != nil && errors.IsNotFound(err) {
			// Create new CronJob
			cronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, cronJobName, targetNamespace, "liveness")
			if err != nil {
				log.Error(err, "Failed to build CronJob")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "Failed to create CronJob")
				return ctrl.Result{}, err
			}
			log.Info("Created new CronJob", "CronJob", cronJobName)
			liveness.Status.State = "Running"
			liveness.Status.LastExecuted = time.Now().Format(time.RFC3339)
			if err := r.Status().Update(ctx, liveness); err != nil {
				log.Error(err, "Failed to update status")
			}
		} else if err != nil {
			log.Error(err, "Failed to get CronJob")
			return ctrl.Result{}, err
		}
	} else {
		// Create one-time Job
		job := &batchv1.Job{}
		jobNN := types.NamespacedName{Name: jobName, Namespace: targetNamespace}

		err := r.Get(ctx, jobNN, job)
		if err != nil && errors.IsNotFound(err) {
			// Create new Job
			job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, jobName, targetNamespace, "liveness")
			if err != nil {
				log.Error(err, "Failed to build Job")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Failed to create Job")
				return ctrl.Result{}, err
			}
			log.Info("Created new Job", "Job", jobName)
			liveness.Status.State = "Running"
			liveness.Status.LastExecuted = time.Now().Format(time.RFC3339)
			if err := r.Status().Update(ctx, liveness); err != nil {
				log.Error(err, "Failed to update status")
			}
		} else if err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// livenessAdapter adapts Liveness to SecurityResource interface
type livenessAdapter struct {
	liveness *securityv1alpha1.Liveness
}

func (a *livenessAdapter) GetName() string {
	return a.liveness.Name
}

func (a *livenessAdapter) GetNamespace() string {
	return a.liveness.Namespace
}

func (a *livenessAdapter) GetSpec() *ResourceSpec {
	envVars := make([]corev1.EnvVar, len(a.liveness.Spec.AdditionalEnv))
	for i, ev := range a.liveness.Spec.AdditionalEnv {
		envVars[i] = corev1.EnvVar{
			Name:  ev.Name,
			Value: ev.Value,
		}
	}
	return &ResourceSpec{
		Tool:          a.liveness.Spec.Tool,
		Target:        a.liveness.Spec.Target,
		Args:          a.liveness.Spec.Args,
		HTTPProxy:     a.liveness.Spec.HTTPProxy,
		AdditionalEnv: envVars,
		Debug:         a.liveness.Spec.Debug,
		Periodic:      a.liveness.Spec.Periodic,
		Schedule:      a.liveness.Spec.Schedule,
	}
}

func (a *livenessAdapter) GetStatus() *ResourceStatus {
	return &ResourceStatus{
		State:        a.liveness.Status.State,
		LastExecuted: a.liveness.Status.LastExecuted,
	}
}

func (a *livenessAdapter) GetKubeObject() client.Object {
	return a.liveness
}

// SetupWithManager sets up the controller with the Manager.
func (r *LivenessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Liveness{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
