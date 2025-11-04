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
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

	// Determine target namespace - use the CR's namespace
	targetNamespace := enum.Namespace

	// Generate names for Job/CronJob - use enum name directly for consistency with Loki labels
	jobName := enum.Name
	cronJobName := fmt.Sprintf("%s-cronjob", enum.Name)

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

	podSpec, _ := r.buildPodSpec(enum)

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

	podSpec, _ := r.buildPodSpec(enum)

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

func (r *EnumerationReconciler) buildPodSpec(enum *securityv1alpha1.Enumeration) (corev1.PodSpec, error) {
	log := log.FromContext(context.Background())

	// Get tool configuration from configurator
	toolConfig, err := r.Configurator.GetToolConfig(enum.Spec.Tool)
	if err != nil {
		log.Error(err, "Failed to get tool configuration", "tool", enum.Spec.Tool)
		return corev1.PodSpec{}, err
	}

	// Build environment variables
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

	// Build command from template using proper template handling
	command, err := r.Configurator.BuildCommand(enum.Spec.Tool, enum.Spec.Target, enum.Spec.Args)
	if err != nil {
		log.Error(err, "Failed to build command from template", "tool", enum.Spec.Tool)
		return corev1.PodSpec{}, err
	}

	// Extract capabilities
	capabilities, _ := r.Configurator.GetCapabilities(enum.Spec.Tool)
	securityContext := &corev1.SecurityContext{}
	if len(capabilities) > 0 {
		capList := make([]corev1.Capability, len(capabilities))
		for i, cap := range capabilities {
			capList[i] = corev1.Capability(cap)
		}
		securityContext.Capabilities = &corev1.Capabilities{
			Add: capList,
		}
	}

	// Build command with shell wrapper for logging
	var shellWrappedCommand string
	if enum.Spec.Debug {
		// Debug mode: output to stdout - pass command directly
		shellWrappedCommand = strings.Join(command, " ")
	} else {
		// Normal mode: redirect to emptydir volume and create done file
		shellWrappedCommand = strings.Join(command, " ") + " > /logs/job.log 2>&1 && touch /logs/done"
	}

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:            "security-tool",
				Image:           toolConfig.Image,
				Command:         []string{"sh"},
				Args:            []string{"-c", shellWrappedCommand},
				Env:             envVars,
				SecurityContext: securityContext,
			},
		},
	}

	// Add volume and sidecar only if not in debug mode
	if !enum.Spec.Debug {
		// Add logs volume
		podSpec.Volumes = []corev1.Volume{
			{
				Name: "logs",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "fluent-bit-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-fluent-bit-config",
						},
					},
				},
			},
		}
		podSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "logs",
				MountPath: "/logs",
			},
		}

		// Add Fluent Bit sidecar
		fluentBitSidecar := r.buildFluentBitSidecar(enum)
		podSpec.Containers = append(podSpec.Containers, fluentBitSidecar)
	}

	return podSpec, nil
}

func (r *EnumerationReconciler) buildFluentBitSidecar(enum *securityv1alpha1.Enumeration) corev1.Container {
	return corev1.Container{
		Name:  "fluent-bit-sidecar",
		Image: "fluent/fluent-bit:latest",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "logs",
				MountPath: "/logs",
				ReadOnly:  true,
			},
			{
				Name:      "fluent-bit-config",
				MountPath: "/fluent-bit/etc",
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "LOKI_HOST",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "loki-host",
					},
				},
			},
			{
				Name: "LOKI_PORT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "loki-port",
					},
				},
			},
			{
				Name: "LOKI_TLS",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "loki-tls",
					},
				},
			},
			{
				Name: "LOKI_TLS_VERIFY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "loki-tls-verify",
					},
				},
			},
			{
				Name: "LOKI_TENANT_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "loki-tenant-id",
					},
				},
			},
			{
				Name: "LOKI_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "loki-user",
					},
				},
			},
			{
				Name: "LOKI_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "loki-password",
					},
				},
			},
			{
				Name: "CLUSTER_NAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kttack-loki-credentials",
						},
						Key: "cluster-name",
					},
				},
			},
			{
				Name:  "NAMESPACE",
				Value: enum.Namespace,
			},
			{
				Name:  "JOB_NAME",
				Value: enum.Name,
			},
			{
				Name:  "TOOL_TYPE",
				Value: enum.Spec.Tool,
			},
		},
		Command:   []string{"/fluent-bit/bin/fluent-bit"},
		Args:      []string{"-c", "/fluent-bit/etc/fluent-bit.conf"},
		Lifecycle: &corev1.Lifecycle{},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(64*1024*1024, resource.BinarySI),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(256*1024*1024, resource.BinarySI),
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
