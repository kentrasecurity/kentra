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

// EnumerationReconciler reconciles a Enumeration object
type EnumerationReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Configurator *ToolsConfigurator
}

//+kubebuilder:rbac:groups=kttack.io,resources=enumerations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=enumerations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=enumerations/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch

// Reconcile implements reconciliation for Enumeration resources
func (r *EnumerationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Load tool configurations from ConfigMap if not already loaded
	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool specifications ConfigMap - controller cannot proceed", "ConfigMap", "tool-specs")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Fetch the Enumeration resource
	enum := &securityv1alpha1.Enumeration{}
	if err := r.Get(ctx, req.NamespacedName, enum); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Enumeration resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Enumeration")
		return ctrl.Result{}, err
	}

	// Determine target namespace
	targetNamespace := enum.Namespace

	// Generate names for Job/CronJob
	jobName := enum.Name
	cronJobName := fmt.Sprintf("%s-cronjob", enum.Name)

	// Convert to SecurityResource adapter
	adapter := &enumerationAdapter{enum}

	// Check if we need to create a job or cronjob
	if enum.Spec.Periodic {
		// Create or update CronJob
		cronJob := &batchv1.CronJob{}
		cronJobNN := types.NamespacedName{Name: cronJobName, Namespace: targetNamespace}

		err := r.Get(ctx, cronJobNN, cronJob)
		if err != nil && errors.IsNotFound(err) {
			// Create new CronJob
			cronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, cronJobName, targetNamespace, "enumeration")
			if err != nil {
				log.Error(err, "Failed to build CronJob")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "Failed to create CronJob")
				return ctrl.Result{}, err
			}
			log.Info("Created new CronJob", "CronJob", cronJobName)
			enum.Status.State = "Running"
			enum.Status.LastExecuted = time.Now().Format(time.RFC3339)
			if err := r.Status().Update(ctx, enum); err != nil {
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
			job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, jobName, targetNamespace, "enumeration")
			if err != nil {
				log.Error(err, "Failed to build Job")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Failed to create Job")
				return ctrl.Result{}, err
			}
			log.Info("Created new Job", "Job", jobName)
			enum.Status.State = "Running"
			enum.Status.LastExecuted = time.Now().Format(time.RFC3339)
			if err := r.Status().Update(ctx, enum); err != nil {
				log.Error(err, "Failed to update status")
			}
		} else if err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// enumerationAdapter adapts Enumeration to SecurityResource interface
type enumerationAdapter struct {
	enum *securityv1alpha1.Enumeration
}

func (a *enumerationAdapter) GetName() string {
	return a.enum.Name
}

func (a *enumerationAdapter) GetNamespace() string {
	return a.enum.Namespace
}

func (a *enumerationAdapter) GetSpec() *ResourceSpec {
	envVars := make([]corev1.EnvVar, len(a.enum.Spec.AdditionalEnv))
	for i, ev := range a.enum.Spec.AdditionalEnv {
		envVars[i] = corev1.EnvVar{
			Name:  ev.Name,
			Value: ev.Value,
		}
	}
	return &ResourceSpec{
		Tool:          a.enum.Spec.Tool,
		Target:        a.enum.Spec.Target,
		Args:          a.enum.Spec.Args,
		HTTPProxy:     a.enum.Spec.HTTPProxy,
		AdditionalEnv: envVars,
		Debug:         a.enum.Spec.Debug,
		Periodic:      a.enum.Spec.Periodic,
		Schedule:      a.enum.Spec.Schedule,
	}
}

func (a *enumerationAdapter) GetStatus() *ResourceStatus {
	return &ResourceStatus{
		State:        a.enum.Status.State,
		LastExecuted: a.enum.Status.LastExecuted,
	}
}

func (a *enumerationAdapter) GetKubeObject() client.Object {
	return a.enum
}

// SetupWithManager sets up the controller with the Manager.
func (r *EnumerationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Enumeration{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
