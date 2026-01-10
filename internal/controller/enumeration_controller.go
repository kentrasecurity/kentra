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

// EnumerationReconciler reconciles a Enumeration object
type EnumerationReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *ToolsConfigurator
	ControllerNamespace string
}

//+kubebuilder:rbac:groups=kentra.sh,resources=enumerations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kentra.sh,resources=enumerations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kentra.sh,resources=enumerations/finalizers,verbs=update
//+kubebuilder:rbac:groups=kentra.sh,resources=targetpools,verbs=get;list;watch
//+kubebuilder:rbac:groups=kentra.sh,resources=storagepools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get

// Reconcile implements reconciliation for Enumeration resources
func (r *EnumerationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Load tool configurations from ConfigMap if not already loaded
	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool specifications ConfigMap - controller cannot proceed", "ConfigMap", "kentra-tool-specs")
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

	// Check if namespace is managed by Kentra
	isManaged, err := isNamespaceManagedByKentra(ctx, r.Client, enum.Namespace)
	if err != nil {
		log.Error(err, "Failed to check if namespace is managed by Kentra", "namespace", enum.Namespace)
		return ctrl.Result{}, err
	}
	if !isManaged {
		log.Error(fmt.Errorf("namespace not managed by Kentra"), "Cannot create Enumeration in namespace without 'managed-by-kentra' annotation", "namespace", enum.Namespace)
		return ctrl.Result{}, fmt.Errorf("namespace %s is not managed by Kentra (missing 'managed-by-kentra' annotation)", enum.Namespace)
	}

	// Ensure labels are set
	if enum.Labels == nil {
		enum.Labels = make(map[string]string)
	}
	needsUpdate := false
	if enum.Labels["kentra.sh/resource-type"] != "attack" {
		enum.Labels["kentra.sh/resource-type"] = "attack"
		needsUpdate = true
	}

	// Update the resource if labels were modified
	if needsUpdate {
		if err := r.Update(ctx, enum); err != nil {
			log.Error(err, "Failed to update Enumeration labels")
			return ctrl.Result{}, err
		}
	}

	// Initialize files slice
	var resolvedFiles []string

	// Resolve StoragePool reference if provided
	if enum.Spec.StoragePool != "" {
		sg := &securityv1alpha1.StoragePool{}
		sgNN := types.NamespacedName{Name: enum.Spec.StoragePool, Namespace: enum.Namespace}
		if err := r.Get(ctx, sgNN, sg); err != nil {
			log.Error(err, "Failed to get referenced StoragePool", "StoragePool", enum.Spec.StoragePool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		// Set files from StoragePool
		resolvedFiles = sg.Spec.Files
		log.Info("Resolved StoragePool", "StoragePool", enum.Spec.StoragePool, "filesCount", len(resolvedFiles))
	}

	// Resolve TargetPool reference if provided
	if enum.Spec.TargetPool != "" {
		tg := &securityv1alpha1.TargetPool{}
		tgNN := types.NamespacedName{Name: enum.Spec.TargetPool, Namespace: enum.Namespace}
		if err := r.Get(ctx, tgNN, tg); err != nil {
			log.Error(err, "Failed to get referenced TargetPool", "TargetPool", enum.Spec.TargetPool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		// Set target and port from TargetPool
		enum.Spec.Target = tg.Spec.Target
		if tg.Spec.Port != "" && enum.Spec.Port == "" {
			enum.Spec.Port = tg.Spec.Port
		}
		// Update resolved status
		enum.Status.ResolvedTarget = tg.Spec.Target
		enum.Status.ResolvedPort = tg.Spec.Port
	} else {
		// Use direct target and port
		enum.Status.ResolvedTarget = enum.Spec.Target
		enum.Status.ResolvedPort = enum.Spec.Port
	}

	// Validate that either Target or TargetPool is set
	if enum.Spec.Target == "" {
		log.Error(fmt.Errorf("neither target nor targetPool specified"), "Invalid Enumeration resource")
		return ctrl.Result{}, fmt.Errorf("Enumeration must have either 'target' or 'targetPool' specified")
	}

	// Determine target namespace
	targetNamespace := enum.Namespace

	// Generate names for Job/CronJob
	jobName := enum.Name
	cronJobName := fmt.Sprintf("%s-cronjob", enum.Name)

	// Convert to SecurityResource adapter
	adapter := &enumerationAdapter{enum, resolvedFiles}

	log.Info("Building resource with spec", "tool", adapter.GetSpec().Tool, "filesCount", len(adapter.GetSpec().Files))

	// Check if we need to create a job or cronjob
	if enum.Spec.Periodic {
		// Create or update CronJob
		cronJob := &batchv1.CronJob{}
		cronJobNN := types.NamespacedName{Name: cronJobName, Namespace: targetNamespace}

		err := r.Get(ctx, cronJobNN, cronJob)
		if err != nil && errors.IsNotFound(err) {
			// Create new CronJob
			cronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, cronJobName, targetNamespace, "enumeration", r.ControllerNamespace)
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
		} else if err != nil {
			log.Error(err, "Failed to get CronJob")
			return ctrl.Result{}, err
		} else {
			// CronJob exists - check if it needs to be recreated due to spec change
			currentGeneration := fmt.Sprintf("%d", enum.Generation)
			existingGeneration := cronJob.Annotations["kentra.sh/parent-generation"]

			if existingGeneration != currentGeneration {
				log.Info("Enumeration spec changed, deleting and recreating CronJob",
					"CronJob", cronJobName,
					"oldGeneration", existingGeneration,
					"newGeneration", currentGeneration)

				// Delete the existing CronJob
				if err := r.Delete(ctx, cronJob); err != nil {
					log.Error(err, "Failed to delete outdated CronJob")
					return ctrl.Result{}, err
				}

				// Create new CronJob with updated spec
				newCronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, cronJobName, targetNamespace, "enumeration", r.ControllerNamespace)
				if err != nil {
					log.Error(err, "Failed to build new CronJob")
					return ctrl.Result{}, err
				}
				if err := r.Create(ctx, newCronJob); err != nil {
					log.Error(err, "Failed to create new CronJob")
					return ctrl.Result{}, err
				}
				log.Info("Recreated CronJob with updated spec", "CronJob", cronJobName)
				enum.Status.State = "Running"
				enum.Status.LastExecuted = time.Now().Format(time.RFC3339)
			}
		}
	} else {
		// Create one-time Job
		job := &batchv1.Job{}
		jobNN := types.NamespacedName{Name: jobName, Namespace: targetNamespace}

		err := r.Get(ctx, jobNN, job)
		if err != nil && errors.IsNotFound(err) {
			// Create new Job
			job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, jobName, targetNamespace, "enumeration", r.ControllerNamespace)
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
		} else if err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		} else {
			// Job exists - check if it needs to be recreated due to spec change
			// Note: Jobs are immutable, so we need to delete and recreate
			currentGeneration := fmt.Sprintf("%d", enum.Generation)
			existingGeneration := job.Annotations["kentra.sh/parent-generation"]

			if existingGeneration != currentGeneration {
				log.Info("Enumeration spec changed, deleting and recreating Job",
					"Job", jobName,
					"oldGeneration", existingGeneration,
					"newGeneration", currentGeneration)

				// Delete the existing Job
				propagationPolicy := metav1.DeletePropagationBackground
				if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil {
					log.Error(err, "Failed to delete outdated Job")
					return ctrl.Result{}, err
				}

				// Note: We don't immediately recreate the Job here because it will be
				// recreated on the next reconciliation after the deletion completes
				log.Info("Deleted outdated Job, will recreate on next reconcile", "Job", jobName)
				return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
			}
		}
	}

	// Update status with resolved target and port
	if err := r.Status().Update(ctx, enum); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// enumerationAdapter adapts Enumeration to SecurityResource interface
type enumerationAdapter struct {
	enum          *securityv1alpha1.Enumeration
	resolvedFiles []string
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
		Category:      a.enum.Spec.Category,
		Args:          a.enum.Spec.Args,
		HTTPProxy:     a.enum.Spec.HTTPProxy,
		AdditionalEnv: envVars,
		Debug:         a.enum.Spec.Debug,
		Periodic:      a.enum.Spec.Periodic,
		Schedule:      a.enum.Spec.Schedule,
		Port:          a.enum.Spec.Port,
		Files:         a.resolvedFiles,
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
