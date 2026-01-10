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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// LivenessReconciler reconciles a Liveness object
type LivenessReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *ToolsConfigurator
	ControllerNamespace string
}

//+kubebuilder:rbac:groups=kentra.sh,resources=livenesses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=livenesses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=livenesses/finalizers,verbs=update
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get

// Reconcile implements reconciliation for Liveness resources
func (r *LivenessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Load tool configurations from ConfigMap if not already loaded
	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool specifications ConfigMap - controller cannot proceed", "ConfigMap", "kentra-tool-specs")
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

	// Check if namespace is managed by Kentra
	isManaged, err := isNamespaceManagedByKentra(ctx, r.Client, liveness.Namespace)
	if err != nil {
		log.Error(err, "Failed to check if namespace is managed by Kentra", "namespace", liveness.Namespace)
		return ctrl.Result{}, err
	}
	if !isManaged {
		log.Error(fmt.Errorf("namespace not managed by Kentra"), "Cannot create Liveness in namespace without 'managed-by-kentra' annotation", "namespace", liveness.Namespace)
		return ctrl.Result{}, fmt.Errorf("namespace %s is not managed by Kentra (missing 'managed-by-kentra' annotation)", liveness.Namespace)
	}

	// Ensure labels are set
	if liveness.Labels == nil {
		liveness.Labels = make(map[string]string)
	}
	needsUpdate := false
	if liveness.Labels["kentra.sh/resource-type"] != "attack" {
		liveness.Labels["kentra.sh/resource-type"] = "attack"
		needsUpdate = true
	}

	// Update the resource if labels were modified
	if needsUpdate {
		if err := r.Update(ctx, liveness); err != nil {
			log.Error(err, "Failed to update Liveness labels")
			return ctrl.Result{}, err
		}
	}

	// Resolve TargetPool reference if provided
	if liveness.Spec.TargetPool != "" {
		tg := &securityv1alpha1.TargetPool{}
		tgNN := types.NamespacedName{Name: liveness.Spec.TargetPool, Namespace: liveness.Namespace}
		if err := r.Get(ctx, tgNN, tg); err != nil {
			log.Error(err, "Failed to get referenced TargetPool", "TargetPool", liveness.Spec.TargetPool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		// Set target and port from TargetPool
		liveness.Spec.Target = tg.Spec.Target
		// Update resolved status
		liveness.Status.ResolvedTarget = tg.Spec.Target
	} else {
		// Use direct target
		liveness.Status.ResolvedTarget = liveness.Spec.Target
	}

	// Validate that either Target or TargetPool is set
	if liveness.Spec.Target == "" {
		log.Error(fmt.Errorf("neither target nor targetPool specified"), "Invalid Liveness resource")
		return ctrl.Result{}, fmt.Errorf("Liveness must have either 'target' or 'targetPool' specified")
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
			cronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, cronJobName, targetNamespace, "liveness", r.ControllerNamespace)
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
		} else {
			// CronJob exists - check if it needs to be recreated due to spec change
			currentGeneration := fmt.Sprintf("%d", liveness.Generation)
			existingGeneration := cronJob.Annotations["kentra.sh/parent-generation"]

			if existingGeneration != currentGeneration {
				log.Info("Liveness spec changed, deleting and recreating CronJob",
					"CronJob", cronJobName,
					"oldGeneration", existingGeneration,
					"newGeneration", currentGeneration)

				// Delete the existing CronJob
				if err := r.Delete(ctx, cronJob); err != nil {
					log.Error(err, "Failed to delete outdated CronJob")
					return ctrl.Result{}, err
				}

				// Create new CronJob with updated spec
				newCronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, cronJobName, targetNamespace, "liveness", r.ControllerNamespace)
				if err != nil {
					log.Error(err, "Failed to build new CronJob")
					return ctrl.Result{}, err
				}
				if err := r.Create(ctx, newCronJob); err != nil {
					log.Error(err, "Failed to create new CronJob")
					return ctrl.Result{}, err
				}
				log.Info("Recreated CronJob with updated spec", "CronJob", cronJobName)
				liveness.Status.State = "Running"
				liveness.Status.LastExecuted = time.Now().Format(time.RFC3339)
				if err := r.Status().Update(ctx, liveness); err != nil {
					log.Error(err, "Failed to update status")
				}
			}
		}
	} else {
		// Create one-time Job
		job := &batchv1.Job{}
		jobNN := types.NamespacedName{Name: jobName, Namespace: targetNamespace}

		err := r.Get(ctx, jobNN, job)
		if err != nil && errors.IsNotFound(err) {
			// Create new Job
			job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, jobName, targetNamespace, "liveness", r.ControllerNamespace)
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
		} else {
			// Job exists - check if it needs to be recreated due to spec change
			currentGeneration := fmt.Sprintf("%d", liveness.Generation)
			existingGeneration := job.Annotations["kentra.sh/parent-generation"]

			if existingGeneration != currentGeneration {
				log.Info("Liveness spec changed, deleting and recreating Job",
					"Job", jobName,
					"oldGeneration", existingGeneration,
					"newGeneration", currentGeneration)

				// Delete the existing Job
				propagationPolicy := metav1.DeletePropagationBackground
				if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil {
					log.Error(err, "Failed to delete outdated Job")
					return ctrl.Result{}, err
				}

				log.Info("Deleted outdated Job, will recreate on next reconcile", "Job", jobName)
				return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
			}
		}
	}

	// Update status with resolved target
	if err := r.Status().Update(ctx, liveness); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
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
		Category:      a.liveness.Spec.Category,
		Args:          a.liveness.Spec.Args,
		HTTPProxy:     a.liveness.Spec.HTTPProxy,
		AdditionalEnv: envVars,
		Debug:         a.liveness.Spec.Debug,
		Periodic:      a.liveness.Spec.Periodic,
		Schedule:      a.liveness.Spec.Schedule,
		Files:         []string{},
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
