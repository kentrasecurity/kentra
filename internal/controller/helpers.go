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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// SecurityResource is an interface that defines the common structure for security-related resources
type SecurityResource interface {
	GetName() string
	GetNamespace() string
	GetSpec() *ResourceSpec
	GetStatus() *ResourceStatus
	GetKubeObject() client.Object
}

// ResourceSpec defines the common spec fields across security resources
type ResourceSpec struct {
	Tool          string
	Target        string
	Category      string
	Args          []string
	HTTPProxy     string
	AdditionalEnv []corev1.EnvVar
	Debug         bool
	Periodic      bool
	Schedule      string
	Port          string
	Files         []string
	Assets        []securityv1alpha1.AssetItem
}

// ResourceStatus defines the common status fields across security resources
type ResourceStatus struct {
	State        string
	LastExecuted string
}

// getResourceType returns the kentra-resource-type label value based on appType
func getResourceType(appType string) string {
	switch appType {
	case "enumeration", "osint", "liveness", "securityattack":
		return "attack"
	case "storagepool":
		return "storage"
	case "targetpool":
		return "target"
	case "assetpool":
		return "asset"
	default:
		return appType
	}
}

// BuildJob creates a Job object for a security resource
func BuildJob(ctx context.Context, res SecurityResource, scheme *runtime.Scheme, configurator *ToolsConfigurator, jobName, namespace, appType string) (*batchv1.Job, error) {
	spec := res.GetSpec()

	labels := map[string]string{
		"app":                  appType,
		"tool":                 spec.Tool,
		"task":                 "job",
		"kentra-resource-type": getResourceType(appType),
	}

	podSpec, err := buildPodSpec(ctx, spec, configurator, res.GetNamespace(), res.GetName(), spec.Debug, "job", appType)
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kentra.sh/target":   spec.Target,
				"kentra.sh/tool":     spec.Tool,
				"kentra.sh/category": spec.Category,
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

	controllerutil.SetControllerReference(res.GetKubeObject(), job, scheme)
	return job, nil
}

// BuildCronJob creates a CronJob object for a security resource
func BuildCronJob(ctx context.Context, res SecurityResource, scheme *runtime.Scheme, configurator *ToolsConfigurator, cronJobName, namespace, appType string) (*batchv1.CronJob, error) {
	spec := res.GetSpec()

	labels := map[string]string{
		"app":                  appType,
		"tool":                 spec.Tool,
		"task":                 "cronjob",
		"kentra-resource-type": getResourceType(appType),
	}

	podSpec, err := buildPodSpec(ctx, spec, configurator, res.GetNamespace(), res.GetName(), spec.Debug, "cronjob", appType)
	if err != nil {
		return nil, err
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kentra.sh/target":   spec.Target,
				"kentra.sh/tool":     spec.Tool,
				"kentra.sh/category": spec.Category,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: spec.Schedule,
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

	controllerutil.SetControllerReference(res.GetKubeObject(), cronJob, scheme)
	return cronJob, nil
}

// buildPodSpec creates the PodSpec for a security resource
func buildPodSpec(ctx context.Context, spec *ResourceSpec, configurator *ToolsConfigurator, namespace, resourceName string, debug bool, taskType, resourceType string) (corev1.PodSpec, error) {
	log := log.FromContext(ctx)

	// Get tool configuration from configurator
	toolConfig, err := configurator.GetToolConfig(spec.Tool)
	if err != nil {
		log.Error(err, "Failed to get tool configuration", "tool", spec.Tool)
		return corev1.PodSpec{}, err
	}

	// Build environment variables
	envVars := []corev1.EnvVar{}

	if spec.HTTPProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: spec.HTTPProxy,
		})
	}

	for _, ev := range spec.AdditionalEnv {
		envVars = append(envVars, corev1.EnvVar{
			Name:  ev.Name,
			Value: ev.Value,
		})
	}

	// Build command from template using proper template handling
	var command []string
	if len(spec.Assets) > 0 {
		// Use asset-aware command building
		log.Info("Building command with assets", "tool", spec.Tool, "assetsCount", len(spec.Assets))
		command, err = configurator.BuildCommandWithAssets(spec.Tool, spec.Assets, spec.Args)
		log.Info("Built command", "command", command)
	} else {
		// Use standard command building
		command, err = configurator.BuildCommand(spec.Tool, spec.Target, spec.Port, spec.Args)
	}

	if err != nil {
		log.Error(err, "Failed to build command from template", "tool", spec.Tool)
		return corev1.PodSpec{}, err
	}

	// Extract capabilities
	capabilities, _ := configurator.GetCapabilities(spec.Tool)
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
	if debug {
		// Debug mode: output to stdout - pass command directly with 5 sec delay
		shellWrappedCommand = "sleep 5 && " + strings.Join(command, " ")
	} else {
		// Normal mode: redirect to emptydir volume and create done file with 5 sec delay
		shellWrappedCommand = "sleep 5 && " + strings.Join(command, " ") + " > /logs/job.log 2>&1; touch /logs/done"
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

	// Initialize volumes and volume mounts
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	// Add logs volume and mount if not in debug mode
	if !debug {
		volumes = append(volumes, corev1.Volume{
			Name: "logs",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		volumes = append(volumes, corev1.Volume{
			Name: "fluent-bit-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "kentra-fluent-bit-config",
					},
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "logs",
			MountPath: "/logs",
		})
	}

	// Add config volume and mount if files are specified
	if len(spec.Files) > 0 {
		volumes = append(volumes, corev1.Volume{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "config",
			MountPath: "/config",
		})
	}

	// Set volumes and mounts on pod spec
	podSpec.Volumes = volumes
	podSpec.Containers[0].VolumeMounts = volumeMounts

	// Add init container if files are specified
	if len(spec.Files) > 0 {
		podSpec.InitContainers = []corev1.Container{
			buildS3FileDownloaderInitContainer(spec.Files),
		}
	}

	// Add Fluent Bit sidecar only if not in debug mode
	if !debug {
		fluentBitSidecar := buildFluentBitSidecar(namespace, resourceName, spec.Tool, taskType, resourceType)
		podSpec.Containers = append(podSpec.Containers, fluentBitSidecar)
	}

	return podSpec, nil
}

// buildS3FileDownloaderInitContainer creates an init container that downloads files from S3
func buildS3FileDownloaderInitContainer(files []string) corev1.Container {
	// Build the script to download files from S3
	// Script uses minio/mc to download files from s3://configs bucket
	var downloadScript strings.Builder
	downloadScript.WriteString("#!/bin/sh\n")
	downloadScript.WriteString("set -e\n")
	downloadScript.WriteString("echo 'Starting S3 file download...'\n")
	downloadScript.WriteString("# Configure minio/mc with credentials\n")
	downloadScript.WriteString("mc alias set s3 http://loki-minio-svc.kentra-system.svc.cluster.local:9000 \"${MINIO_ROOT_USER}\" \"${MINIO_ROOT_PASSWORD}\" --api S3v4\n")

	for _, file := range files {
		downloadScript.WriteString(fmt.Sprintf("echo 'Downloading %s...'\n", file))
		downloadScript.WriteString(fmt.Sprintf("mc cp s3/configs/%s /config/%s\n", file, file))
	}

	downloadScript.WriteString("echo 'S3 file download completed successfully'\n")

	return corev1.Container{
		Name:  "s3-file-downloader",
		Image: "minio/mc:latest",
		Command: []string{
			"sh",
			"-c",
			downloadScript.String(),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config",
				MountPath: "/config",
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "MINIO_ROOT_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "loki-minio",
						},
						Key: "rootUser",
					},
				},
			},
			{
				Name: "MINIO_ROOT_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "loki-minio",
						},
						Key: "rootPassword",
					},
				},
			},
		},
	}
}

// buildFluentBitSidecar creates the Fluent Bit sidecar container
func buildFluentBitSidecar(namespace, resourceName, toolType, taskType, resourceType string) corev1.Container {
	return corev1.Container{
		Name:  "fluent-bit-sidecar",
		Image: "percona/fluentbit:4.0.1",
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
							Name: "kentra-loki-credentials",
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
							Name: "kentra-loki-credentials",
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
							Name: "kentra-loki-credentials",
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
							Name: "kentra-loki-credentials",
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
							Name: "kentra-loki-credentials",
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
							Name: "kentra-loki-credentials",
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
							Name: "kentra-loki-credentials",
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
							Name: "kentra-loki-credentials",
						},
						Key: "cluster-name",
					},
				},
			},
			{
				Name:  "NAMESPACE",
				Value: namespace,
			},
			{
				Name:  "JOB_NAME",
				Value: resourceName,
			},
			{
				Name:  "TOOL_TYPE",
				Value: toolType,
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "TASK_TYPE",
				Value: taskType,
			},
			{
				Name:  "RESOURCE_TYPE",
				Value: resourceType,
			},
		},
		Command:   []string{"sh"},
		Args:      []string{"-c", `/opt/fluent-bit/bin/fluent-bit -c /fluent-bit/etc/fluent-bit.conf & PID=$!; while [ ! -f /logs/done ]; do sleep 1; done; sleep 5; kill $PID; wait $PID 2>/dev/null || true`},
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

// UpdateResourceStatus updates the status of a security resource
func UpdateResourceStatus(ctx context.Context, statusWriter client.StatusWriter, res client.Object, state string) {
	log := log.FromContext(ctx)

	// Update status based on type
	switch v := res.(type) {
	case *securityv1alpha1.Enumeration:
		v.Status.State = state
		v.Status.LastExecuted = time.Now().Format(time.RFC3339)
	case *securityv1alpha1.Liveness:
		v.Status.State = state
		v.Status.LastExecuted = time.Now().Format(time.RFC3339)
	case *securityv1alpha1.SecurityAttack:
		v.Status.State = state
		v.Status.LastExecuted = time.Now().Format(time.RFC3339)
	case *securityv1alpha1.Osint:
		v.Status.State = state
		v.Status.LastExecuted = time.Now().Format(time.RFC3339)
	}

	if err := statusWriter.Update(ctx, res); err != nil {
		log.Error(err, "Failed to update resource status")
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}
