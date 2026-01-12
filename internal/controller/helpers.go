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

// isNamespaceManagedByKentra checks if a namespace has the managed-by-kentra annotation
func isNamespaceManagedByKentra(ctx context.Context, c client.Client, namespace string) (bool, error) {
	log := log.FromContext(ctx)

	ns := &corev1.Namespace{}
	if err := c.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		log.Error(err, "Failed to get namespace", "namespace", namespace)
		return false, err
	}

	// Check for managed-by-kentra annotation
	if annotations := ns.GetAnnotations(); annotations != nil {
		if _, ok := annotations["managed-by-kentra"]; ok {
			return true, nil
		}
	}

	return false, nil
}

// copyConfigMapToNamespace copies a ConfigMap from source namespace to target namespace
func copyConfigMapToNamespace(ctx context.Context, c client.Client, sourceNamespace, targetNamespace, labelKey, labelValue string) (string, error) {
	log := log.FromContext(ctx)

	// Find the ConfigMap in the source namespace
	cmList := &corev1.ConfigMapList{}
	if err := c.List(ctx, cmList,
		client.InNamespace(sourceNamespace),
		client.MatchingLabels{labelKey: labelValue},
	); err != nil {
		log.Error(err, "Failed to list ConfigMaps in source namespace", "sourceNamespace", sourceNamespace, "Label", labelKey+"="+labelValue)
		return "", err
	}

	if len(cmList.Items) == 0 {
		return "", fmt.Errorf("no ConfigMap found with label %s=%s in source namespace %s", labelKey, labelValue, sourceNamespace)
	}

	sourceCM := &cmList.Items[0]
	log.Info("Found ConfigMap in source namespace, copying to target", "name", sourceCM.Name, "sourceNamespace", sourceNamespace, "targetNamespace", targetNamespace)

	// Create a new ConfigMap in the target namespace
	targetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sourceCM.Name,
			Namespace:   targetNamespace,
			Labels:      sourceCM.Labels,
			Annotations: sourceCM.Annotations,
		},
		Data:       sourceCM.Data,
		BinaryData: sourceCM.BinaryData,
	}

	// Create the ConfigMap in the target namespace
	if err := c.Create(ctx, targetCM); err != nil {
		log.Error(err, "Failed to create ConfigMap in target namespace", "name", targetCM.Name, "targetNamespace", targetNamespace)
		return "", err
	}

	log.Info("Successfully copied ConfigMap to target namespace", "name", targetCM.Name, "targetNamespace", targetNamespace)
	return targetCM.Name, nil
}

// copySecretToNamespace copies a Secret from source namespace to target namespace
func copySecretToNamespace(ctx context.Context, c client.Client, sourceNamespace, targetNamespace, labelKey, labelValue string) (string, error) {
	log := log.FromContext(ctx)

	// Find the Secret in the source namespace
	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList,
		client.InNamespace(sourceNamespace),
		client.MatchingLabels{labelKey: labelValue},
	); err != nil {
		log.Error(err, "Failed to list Secrets in source namespace", "sourceNamespace", sourceNamespace, "Label", labelKey+"="+labelValue)
		return "", err
	}

	if len(secretList.Items) == 0 {
		return "", fmt.Errorf("no Secret found with label %s=%s in source namespace %s", labelKey, labelValue, sourceNamespace)
	}

	sourceSecret := &secretList.Items[0]
	log.Info("Found Secret in source namespace, copying to target", "name", sourceSecret.Name, "sourceNamespace", sourceNamespace, "targetNamespace", targetNamespace)

	// Create a new Secret in the target namespace
	targetSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sourceSecret.Name,
			Namespace:   targetNamespace,
			Labels:      sourceSecret.Labels,
			Annotations: sourceSecret.Annotations,
		},
		Type:       sourceSecret.Type,
		Data:       sourceSecret.Data,
		StringData: sourceSecret.StringData,
	}

	// Create the Secret in the target namespace
	if err := c.Create(ctx, targetSecret); err != nil {
		log.Error(err, "Failed to create Secret in target namespace", "name", targetSecret.Name, "targetNamespace", targetNamespace)
		return "", err
	}

	log.Info("Successfully copied Secret to target namespace", "name", targetSecret.Name, "targetNamespace", targetNamespace)
	return targetSecret.Name, nil
}

// getConfigMapByLabel finds a ConfigMap by label in the given namespace
// If not found and the namespace is managed by Kentra, it copies from the controller namespace
func getConfigMapByLabel(ctx context.Context, c client.Client, namespace, labelKey, labelValue, controllerNamespace string) (string, error) {
	log := log.FromContext(ctx)

	cmList := &corev1.ConfigMapList{}
	if err := c.List(ctx, cmList,
		client.InNamespace(namespace),
		client.MatchingLabels{labelKey: labelValue},
	); err != nil {
		log.Error(err, "Failed to list ConfigMaps", "Namespace", namespace, "Label", labelKey+"="+labelValue)
		return "", err
	}

	if len(cmList.Items) == 0 {
		// ConfigMap not found in target namespace
		log.Info("ConfigMap not found in target namespace", "namespace", namespace, "label", labelKey+"="+labelValue)

		// Check if namespace is managed by Kentra
		isManaged, err := isNamespaceManagedByKentra(ctx, c, namespace)
		if err != nil {
			return "", fmt.Errorf("failed to check if namespace is managed by Kentra: %w", err)
		}

		if !isManaged {
			return "", fmt.Errorf("namespace %s is not managed by Kentra (missing 'managed-by-kentra' annotation)", namespace)
		}

		log.Info("Namespace is managed by Kentra, attempting to copy ConfigMap from controller namespace", "namespace", namespace, "controllerNamespace", controllerNamespace)

		// Copy ConfigMap from controller namespace to target namespace
		return copyConfigMapToNamespace(ctx, c, controllerNamespace, namespace, labelKey, labelValue)
	}

	if len(cmList.Items) > 1 {
		log.Info("Multiple ConfigMaps found with same label, using first one", "Count", len(cmList.Items), "Label", labelKey+"="+labelValue)
	}

	return cmList.Items[0].Name, nil
}

// getSecretByLabel finds a Secret by label in the given namespace
// If not found and the namespace is managed by Kentra, it copies from the controller namespace
func getSecretByLabel(ctx context.Context, c client.Client, namespace, labelKey, labelValue, controllerNamespace string) (string, error) {
	log := log.FromContext(ctx)

	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList,
		client.InNamespace(namespace),
		client.MatchingLabels{labelKey: labelValue},
	); err != nil {
		log.Error(err, "Failed to list Secrets", "Namespace", namespace, "Label", labelKey+"="+labelValue)
		return "", err
	}

	if len(secretList.Items) == 0 {
		// Secret not found in target namespace
		log.Info("Secret not found in target namespace", "namespace", namespace, "label", labelKey+"="+labelValue)

		// Check if namespace is managed by Kentra
		isManaged, err := isNamespaceManagedByKentra(ctx, c, namespace)
		if err != nil {
			return "", fmt.Errorf("failed to check if namespace is managed by Kentra: %w", err)
		}

		if !isManaged {
			return "", fmt.Errorf("namespace %s is not managed by Kentra (missing 'managed-by-kentra' annotation)", namespace)
		}

		log.Info("Namespace is managed by Kentra, attempting to copy Secret from controller namespace", "namespace", namespace, "controllerNamespace", controllerNamespace)

		// Copy Secret from controller namespace to target namespace
		return copySecretToNamespace(ctx, c, controllerNamespace, namespace, labelKey, labelValue)
	}

	if len(secretList.Items) > 1 {
		log.Info("Multiple Secrets found with same label, using first one", "Count", len(secretList.Items), "Label", labelKey+"="+labelValue)
	}

	return secretList.Items[0].Name, nil
}

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
	Module        string
	Payload       string
	RawCommand    string
	Args          []string
	HTTPProxy     string
	AdditionalEnv []corev1.EnvVar
	Capabilities  []string
	Debug         bool
	Periodic      bool
	Schedule      string
	Port          string
	Files         []string
	Assets        []securityv1alpha1.AssetItem
	ReverseShell  *securityv1alpha1.ReverseShellConfig
}

// ResourceStatus defines the common status fields across security resources
type ResourceStatus struct {
	State        string
	LastExecuted string
}

// getResourceType returns the kentra.sh/resource-type label value based on appType
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
func BuildJob(ctx context.Context, res SecurityResource, scheme *runtime.Scheme, configurator *ToolsConfigurator, c client.Client, jobName, namespace, appType, controllerNamespace string) (*batchv1.Job, error) {
	spec := res.GetSpec()

	labels := map[string]string{
		"app":                     appType,
		"tool":                    spec.Tool,
		"task":                    "job",
		"kentra.sh/resource-type": getResourceType(appType),
	}

	podSpec, err := buildPodSpec(ctx, c, spec, configurator, res.GetNamespace(), res.GetName(), spec.Debug, "job", appType, controllerNamespace)
	if err != nil {
		return nil, err
	}

	// Get the generation from the parent resource
	generation := res.GetKubeObject().GetGeneration()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kentra.sh/target":            spec.Target,
				"kentra.sh/tool":              spec.Tool,
				"kentra.sh/category":          spec.Category,
				"kentra.sh/parent-generation": fmt.Sprintf("%d", generation),
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

	if err := controllerutil.SetControllerReference(res.GetKubeObject(), job, scheme); err != nil {
		return nil, err
	}
	return job, nil
}

// BuildCronJob creates a CronJob object for a security resource
func BuildCronJob(ctx context.Context, res SecurityResource, scheme *runtime.Scheme, configurator *ToolsConfigurator, c client.Client, cronJobName, namespace, appType, controllerNamespace string) (*batchv1.CronJob, error) {
	spec := res.GetSpec()

	labels := map[string]string{
		"app":                     appType,
		"tool":                    spec.Tool,
		"task":                    "cronjob",
		"kentra.sh/resource-type": getResourceType(appType),
	}

	podSpec, err := buildPodSpec(ctx, c, spec, configurator, res.GetNamespace(), res.GetName(), spec.Debug, "cronjob", appType, controllerNamespace)
	if err != nil {
		return nil, err
	}

	// Get the generation from the parent resource
	generation := res.GetKubeObject().GetGeneration()

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kentra.sh/target":            spec.Target,
				"kentra.sh/tool":              spec.Tool,
				"kentra.sh/category":          spec.Category,
				"kentra.sh/parent-generation": fmt.Sprintf("%d", generation),
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

	if err := controllerutil.SetControllerReference(res.GetKubeObject(), cronJob, scheme); err != nil {
		return nil, err
	}
	return cronJob, nil
}

// buildPodSpec creates the PodSpec for a security resource
func buildPodSpec(ctx context.Context, c client.Client, spec *ResourceSpec, configurator *ToolsConfigurator, namespace, resourceName string, debug bool, taskType, resourceType, controllerNamespace string) (corev1.PodSpec, error) {
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
	if spec.RawCommand != "" {
		// Use raw command as-is for rawExploit category
		log.Info("Using raw command", "tool", spec.Tool, "command", spec.RawCommand)
		command = []string{"/bin/sh", "-c", spec.RawCommand}
	} else if len(spec.Assets) > 0 {
		// Use asset-aware command building
		log.Info("Building command with assets", "tool", spec.Tool, "assetsCount", len(spec.Assets))
		command, err = configurator.BuildCommandWithAssets(spec.Tool, spec.Assets, spec.Args)
		log.Info("Built command", "command", command)
	} else if spec.Module != "" {
		// Use module/payload-aware command building for exploits
		log.Info("Building command with module", "tool", spec.Tool, "module", spec.Module, "payload", spec.Payload)
		command, err = configurator.BuildCommandWithModule(spec.Tool, spec.Target, spec.Port, spec.Module, spec.Payload, spec.Args)
		log.Info("Built command", "command", command)
	} else {
		// Use standard command building
		command, err = configurator.BuildCommand(spec.Tool, spec.Target, spec.Port, spec.Args)
	}

	if err != nil {
		log.Error(err, "Failed to build command from template", "tool", spec.Tool)
		return corev1.PodSpec{}, err
	}

	// For rawExploit, use custom capabilities and skip tool config processing
	var securityContext *corev1.SecurityContext
	if spec.RawCommand != "" {
		// rawExploit: use spec.Capabilities instead of tool capabilities
		securityContext = &corev1.SecurityContext{}
		if len(spec.Capabilities) > 0 {
			capList := make([]corev1.Capability, len(spec.Capabilities))
			for i, cap := range spec.Capabilities {
				capList[i] = corev1.Capability(cap)
			}
			securityContext.Capabilities = &corev1.Capabilities{
				Add: capList,
			}
		}
	} else {
		// Normal exploit: extract capabilities from tool config
		securityContext = &corev1.SecurityContext{}
		toolCapabilities, _ := configurator.GetCapabilities(spec.Tool)
		if len(toolCapabilities) > 0 {
			capList := make([]corev1.Capability, len(toolCapabilities))
			for i, cap := range toolCapabilities {
				capList[i] = corev1.Capability(cap)
			}
			securityContext.Capabilities = &corev1.Capabilities{
				Add: capList,
			}
		}
	}

	// Build command with shell wrapper for logging
	var shellWrappedCommand string
	var containerCmd []string
	var containerArgs []string

	if spec.RawCommand != "" {
		// For rawExploit: execute rawCommand directly without wrapping
		containerCmd = []string{"/bin/sh", "-c"}
		containerArgs = []string{spec.RawCommand}
	} else {
		// For other exploits: apply shell wrapping with sleep delay
		if debug {
			// Debug mode: output to stdout - pass command directly with 5 sec delay
			if len(command) > 0 && (command[0] == "sh" || command[0] == "/bin/sh") && len(command) > 2 && command[1] == "-c" {
				// Already shell-wrapped raw command
				shellWrappedCommand = "sleep 5 && " + command[2]
			} else {
				shellWrappedCommand = "sleep 5 && " + strings.Join(command, " ")
			}
		} else {
			// Normal mode: redirect to emptydir volume and create done file with 5 sec delay
			if len(command) > 0 && (command[0] == "sh" || command[0] == "/bin/sh") && len(command) > 2 && command[1] == "-c" {
				// Already shell-wrapped raw command
				shellWrappedCommand = "sleep 5 && " + command[2] + " > /logs/job.log 2>&1; touch /logs/done"
			} else {
				shellWrappedCommand = "sleep 5 && " + strings.Join(command, " ") + " > /logs/job.log 2>&1; touch /logs/done"
			}
		}
		containerCmd = []string{"sh"}
		containerArgs = []string{"-c", shellWrappedCommand}
	}

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:            "security-tool",
				Image:           toolConfig.Image,
				Command:         containerCmd,
				Args:            containerArgs,
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

		// Find fluent-bit config by label
		fluentBitConfigName, err := getConfigMapByLabel(ctx, c, namespace, "kentra.sh/resource-type", "fluentbit-config", controllerNamespace)
		if err != nil {
			log.Error(err, "Failed to find fluent-bit ConfigMap", "namespace", namespace)
			return corev1.PodSpec{}, fmt.Errorf("failed to find fluent-bit ConfigMap: %w", err)
		}

		volumes = append(volumes, corev1.Volume{
			Name: "fluent-bit-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fluentBitConfigName,
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

	// Add init containers for files
	initContainers := []corev1.Container{}

	if len(spec.Files) > 0 {
		initContainers = append(initContainers, buildS3FileDownloaderInitContainer(spec.Files))
	}

	if len(initContainers) > 0 {
		podSpec.InitContainers = initContainers
	}

	// Add reverse shell sidecar container if enabled
	if spec.ReverseShell != nil && spec.ReverseShell.Enabled {
		revshellContainer := buildReverseShellHandlerSidecar(ctx, spec, configurator, namespace, resourceName)
		podSpec.Containers = append(podSpec.Containers, revshellContainer)
		podSpec.InitContainers = initContainers
	}

	// Add Fluent Bit sidecar only if not in debug mode
	if !debug {
		fluentBitSidecar, err := buildFluentBitSidecar(ctx, c, namespace, resourceName, spec.Tool, taskType, resourceType, controllerNamespace)
		if err != nil {
			log.Error(err, "Failed to build fluent-bit sidecar", "namespace", namespace)
			return corev1.PodSpec{}, fmt.Errorf("failed to build fluent-bit sidecar: %w", err)
		}
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

// buildReverseShellHandlerInitContainer creates an init container that spawns a reverse shell handler
func buildReverseShellHandlerInitContainer(ctx context.Context, spec *ResourceSpec, configurator *ToolsConfigurator, namespace, resourceName string) corev1.Container {
	log := log.FromContext(ctx)

	// Build the reverse shell handler command using metasploit
	handlerArgs := []string{
		fmt.Sprintf("PAYLOAD=%s", spec.ReverseShell.Payload),
		fmt.Sprintf("LHOST=%s", spec.ReverseShell.Host),
		fmt.Sprintf("LPORT=%s", spec.ReverseShell.Port),
	}

	handlerCmd, err := configurator.BuildCommandWithModule("metasploit", "", spec.ReverseShell.Port, "exploit/multi/handler", spec.ReverseShell.Payload, handlerArgs)
	if err != nil {
		log.Error(err, "Failed to build reverse shell handler command")
		handlerCmd = []string{
			"/usr/src/metasploit-framework/msfconsole",
			"-q",
			"-x",
			fmt.Sprintf("use exploit/multi/handler ; set PAYLOAD %s; set LHOST %s; set LPORT %s; exploit; exit -y", spec.ReverseShell.Payload, spec.ReverseShell.Host, spec.ReverseShell.Port),
		}
	}

	return corev1.Container{
		Name:    "reverse-shell-handler",
		Image:   "metasploitframework/metasploit-framework:latest",
		Command: []string{"sh", "-c"},
		Args:    []string{strings.Join(handlerCmd, " ")},
		Env: []corev1.EnvVar{
			{
				Name:  "LHOST",
				Value: spec.ReverseShell.Host,
			},
			{
				Name:  "LPORT",
				Value: spec.ReverseShell.Port,
			},
			{
				Name:  "PAYLOAD",
				Value: spec.ReverseShell.Payload,
			},
		},
	}
}

// buildReverseShellHandlerSidecar creates a sidecar container that spawns a reverse shell handler
// This sidecar runs in parallel with the main exploit container, ensuring the handler is ready
func buildReverseShellHandlerSidecar(ctx context.Context, spec *ResourceSpec, configurator *ToolsConfigurator, namespace, resourceName string) corev1.Container {
	log := log.FromContext(ctx)

	// Build the reverse shell handler command using metasploit with exploit mode (stays running)
	handlerCmd := fmt.Sprintf(
		"/usr/src/metasploit-framework/msfconsole -q -x 'use exploit/multi/handler ; set PAYLOAD %s; set LHOST %s; set LPORT %s; exploit'",
		spec.ReverseShell.Payload,
		spec.ReverseShell.Host,
		spec.ReverseShell.Port,
	)

	log.Info("Building reverse shell handler sidecar", "host", spec.ReverseShell.Host, "port", spec.ReverseShell.Port, "payload", spec.ReverseShell.Payload)

	return corev1.Container{
		Name:    "reverse-shell-handler",
		Image:   "metasploitframework/metasploit-framework:latest",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{handlerCmd},
		Env: []corev1.EnvVar{
			{
				Name:  "LHOST",
				Value: spec.ReverseShell.Host,
			},
			{
				Name:  "LPORT",
				Value: spec.ReverseShell.Port,
			},
			{
				Name:  "PAYLOAD",
				Value: spec.ReverseShell.Payload,
			},
		},
		// Container stays running as long as the handler is active
		TTY:   true,
		Stdin: true,
		// No restart policy - container completes when handler exits
	}
}

// buildFluentBitSidecar creates the Fluent Bit sidecar container
func buildFluentBitSidecar(ctx context.Context, c client.Client, namespace, resourceName, toolType, taskType, resourceType, controllerNamespace string) (corev1.Container, error) {
	log := log.FromContext(ctx)

	// Find Loki credentials secret by label
	lokiSecretName, err := getSecretByLabel(ctx, c, namespace, "kentra.sh/resource-type", "loki-credentials", controllerNamespace)
	if err != nil {
		log.Error(err, "Failed to find Loki credentials Secret", "namespace", namespace)
		return corev1.Container{}, fmt.Errorf("failed to find Loki credentials Secret: %w", err)
	}

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
							Name: lokiSecretName,
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
							Name: lokiSecretName,
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
							Name: lokiSecretName,
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
							Name: lokiSecretName,
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
							Name: lokiSecretName,
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
							Name: lokiSecretName,
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
							Name: lokiSecretName,
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
							Name: lokiSecretName,
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
	}, nil
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
