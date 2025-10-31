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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/yourorg/security-operator/api/v1alpha1"
)

// EnumerationReconciler reconciles a Enumeration object
type EnumerationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kttack.io,resources=enumerations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=enumerations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=enumerations/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps,verbs=get;list;watch

// Reconcile implements reconciliation for Enumeration resources
func (r *EnumerationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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
	targetNamespace := enum.Spec.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = "kttack-system"
	}

	// Generate names for Job/CronJob
	baseName := fmt.Sprintf("enum-%s", enum.Name)
	jobName := fmt.Sprintf("%s-job-%d", baseName, time.Now().Unix())
	cronJobName := fmt.Sprintf("%s-cronjob", baseName)

	// Check if we need to create a job or cronjob
	if enum.Spec.Periodic {
		// Create or update CronJob
		cronJob := &batchv1.CronJob{}
		cronJobNN := types.NamespacedName{Name: cronJobName, Namespace: targetNamespace}

		err := r.Get(ctx, cronJobNN, cronJob)
		if err != nil && errors.IsNotFound(err) {
			// Create new CronJob
			cronJob = r.buildCronJob(enum, cronJobName, targetNamespace)
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "Failed to create CronJob")
				return ctrl.Result{}, err
			}
			log.Info("Created new CronJob", "CronJob", cronJobName)
			r.updateStatus(ctx, enum, "Running", "CronJob created")
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
			job = r.buildJob(enum, jobName, targetNamespace)
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Failed to create Job")
				return ctrl.Result{}, err
			}
			log.Info("Created new Job", "Job", jobName)
			r.updateStatus(ctx, enum, "Running", "Job created")
		} else if err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *EnumerationReconciler) buildJob(enum *securityv1alpha1.Enumeration, jobName, namespace string) *batchv1.Job {
	labels := map[string]string{
		"app":  "enumeration",
		"tool": enum.Spec.Tool,
	}

	podSpec := r.buildPodSpec(enum)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kttack.io/target": enum.Spec.Target,
				"kttack.io/tool":   enum.Spec.Tool,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
			BackoffLimit: int32Ptr(2),
		},
	}

	controllerutil.SetControllerReference(enum, job, r.Scheme)
	return job
}

func (r *EnumerationReconciler) buildCronJob(enum *securityv1alpha1.Enumeration, cronJobName, namespace string) *batchv1.CronJob {
	labels := map[string]string{
		"app":  "enumeration",
		"tool": enum.Spec.Tool,
	}

	podSpec := r.buildPodSpec(enum)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kttack.io/target": enum.Spec.Target,
				"kttack.io/tool":   enum.Spec.Tool,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: enum.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec:       podSpec,
					},
					BackoffLimit: int32Ptr(2),
				},
			},
		},
	}

	controllerutil.SetControllerReference(enum, cronJob, r.Scheme)
	return cronJob
}

func (r *EnumerationReconciler) buildPodSpec(enum *securityv1alpha1.Enumeration) corev1.PodSpec {
	command := []string{enum.Spec.Tool}
	args := append([]string{enum.Spec.Target}, enum.Spec.Args...)

	envVars := []corev1.EnvVar{}

	if enum.Spec.HTTPProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: enum.Spec.HTTPProxy,
		})
	}

	for _, ev := range enum.Spec.AdditionalEnv {
		envVars = append(envVars, corev1.EnvVar{
			Name:  ev.Name,
			Value: ev.Value,
		})
	}

	return corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:    enum.Spec.Tool,
				Image:   fmt.Sprintf("%s:latest", enum.Spec.Tool),
				Command: command,
				Args:    args,
				Env:     envVars,
			},
		},
	}
}

func (r *EnumerationReconciler) updateStatus(ctx context.Context, enum *securityv1alpha1.Enumeration, state, message string) {
	log := log.FromContext(ctx)

	enum.Status.State = state
	enum.Status.LastExecuted = time.Now().Format(time.RFC3339)

	if err := r.Status().Update(ctx, enum); err != nil {
		log.Error(err, "Failed to update Enumeration status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *EnumerationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Enumeration{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
