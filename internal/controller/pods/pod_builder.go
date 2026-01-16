package pods

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	"github.com/kentrasecurity/kentra/internal/controller/config"
)

// BuildPodSpec creates a PodSpec for attack containers
// This is the SINGLE place where pod logic lives - no duplication!
func BuildPodSpec(
	ctx context.Context,
	c client.Client,
	tool, target, port string,
	args []string,
	httpProxy string,
	additionalEnv []corev1.EnvVar,
	debug bool,
	files []string,
	assets []securityv1alpha1.AssetItem,
	configurator *config.ToolsConfigurator,
	namespace, resourceName, resourceType, controllerNamespace string,
) (corev1.PodSpec, error) {
	// Get tool config
	toolConfig, err := configurator.GetToolConfig(tool)
	if err != nil {
		return corev1.PodSpec{}, err
	}

	// Build environment variables
	envVars := buildEnvVars(httpProxy, additionalEnv)

	// Build command
	var command []string
	if len(assets) > 0 {
		command, err = configurator.BuildCommandWithAssets(tool, assets, args)
	} else {
		command, err = configurator.BuildCommand(tool, target, port, args)
	}
	if err != nil {
		return corev1.PodSpec{}, err
	}

	// Wrap command
	containerCmd, containerArgs := wrapCommand(command, debug)

	// Build pod spec
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:    "security-tool",
				Image:   toolConfig.Image,
				Command: containerCmd,
				Args:    containerArgs,
				Env:     envVars,
			},
		},
	}

	// Add volumes and mounts
	if err := addVolumes(ctx, c, &podSpec, namespace, debug, files, controllerNamespace); err != nil {
		return corev1.PodSpec{}, err
	}

	// Add sidecars
	if !debug {
		fluentBit, err := buildFluentBitSidecar(ctx, c, namespace, resourceName, tool, resourceType, controllerNamespace)
		if err != nil {
			return corev1.PodSpec{}, err
		}
		podSpec.Containers = append(podSpec.Containers, fluentBit)
	}

	// Add init containers
	if len(files) > 0 {
		podSpec.InitContainers = []corev1.Container{buildS3Downloader(files)}
	}

	return podSpec, nil
}

func buildEnvVars(httpProxy string, additional []corev1.EnvVar) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}
	if httpProxy != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "HTTP_PROXY", Value: httpProxy})
	}
	envVars = append(envVars, additional...)
	return envVars
}

func wrapCommand(command []string, debug bool) ([]string, []string) {
	var shellWrappedCommand string

	if debug {
		shellWrappedCommand = "sleep 5 && " + strings.Join(command, " ")
	} else {
		shellWrappedCommand = "sleep 5 && " + strings.Join(command, " ") + " > /logs/job.log 2>&1; touch /logs/done"
	}

	return []string{"sh"}, []string{"-c", shellWrappedCommand}
}

func addVolumes(ctx context.Context, c client.Client, podSpec *corev1.PodSpec, namespace string, debug bool, files []string, controllerNamespace string) error {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	// Logs volume (if not debug)
	if !debug {
		volumes = append(volumes, corev1.Volume{
			Name: "logs",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		// FluentBit config
		configName, err := getConfigMapByLabel(ctx, c, namespace, "kentra.sh/resource-type", "fluentbit-config", controllerNamespace)
		if err != nil {
			return err
		}

		volumes = append(volumes, corev1.Volume{
			Name: "fluent-bit-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configName},
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "logs",
			MountPath: "/logs",
		})
	}

	// Config volume (if files)
	if len(files) > 0 {
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

	podSpec.Volumes = volumes
	podSpec.Containers[0].VolumeMounts = volumeMounts

	return nil
}

func buildS3Downloader(files []string) corev1.Container {
	var script strings.Builder
	script.WriteString("#!/bin/sh\nset -e\n")
	script.WriteString("mc alias set s3 http://loki-minio-svc.kentra-system.svc.cluster.local:9000 \"${MINIO_ROOT_USER}\" \"${MINIO_ROOT_PASSWORD}\" --api S3v4\n")
	for _, file := range files {
		script.WriteString(fmt.Sprintf("mc cp s3/configs/%s /config/%s\n", file, file))
	}

	return corev1.Container{
		Name:    "s3-file-downloader",
		Image:   "minio/mc:latest",
		Command: []string{"sh", "-c", script.String()},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "config", MountPath: "/config"},
		},
		Env: []corev1.EnvVar{
			{
				Name: "MINIO_ROOT_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "loki-minio"},
						Key:                  "rootUser",
					},
				},
			},
			{
				Name: "MINIO_ROOT_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "loki-minio"},
						Key:                  "rootPassword",
					},
				},
			},
		},
	}
}

func buildFluentBitSidecar(ctx context.Context, c client.Client, namespace, resourceName, tool, resourceType, controllerNamespace string) (corev1.Container, error) {
	secretName, err := getSecretByLabel(ctx, c, namespace, "kentra.sh/resource-type", "loki-credentials", controllerNamespace)
	if err != nil {
		return corev1.Container{}, err
	}

	return corev1.Container{
		Name:  "fluent-bit-sidecar",
		Image: "percona/fluentbit:4.0.1",
		VolumeMounts: []corev1.VolumeMount{
			{Name: "logs", MountPath: "/logs", ReadOnly: true},
			{Name: "fluent-bit-config", MountPath: "/fluent-bit/etc", ReadOnly: true},
		},
		Env:     buildFluentBitEnv(secretName, namespace, resourceName, tool, resourceType),
		Command: []string{"sh"},
		Args:    []string{"-c", `/opt/fluent-bit/bin/fluent-bit -c /fluent-bit/etc/fluent-bit.conf & PID=$!; while [ ! -f /logs/done ]; do sleep 1; done; sleep 5; kill $PID; wait $PID 2>/dev/null || true`},
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

func buildFluentBitEnv(secretName, namespace, resourceName, tool, resourceType string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "LOKI_HOST", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "loki-host"}}},
		{Name: "LOKI_PORT", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "loki-port"}}},
		{Name: "LOKI_TLS", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "loki-tls"}}},
		{Name: "LOKI_TLS_VERIFY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "loki-tls-verify"}}},
		{Name: "LOKI_TENANT_ID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "loki-tenant-id"}}},
		{Name: "LOKI_USER", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "loki-user"}}},
		{Name: "LOKI_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "loki-password"}}},
		{Name: "CLUSTER_NAME", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "cluster-name"}}},
		{Name: "NAMESPACE", Value: namespace},
		{Name: "JOB_NAME", Value: resourceName},
		{Name: "TOOL_TYPE", Value: tool},
		{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		{Name: "RESOURCE_TYPE", Value: resourceType},
	}
}

// Helper functions (copied from helpers.go - only what we need)
func getConfigMapByLabel(ctx context.Context, c client.Client, namespace, key, value, controllerNS string) (string, error) {
	// Implementation from your helpers.go - simplified for brevity
	return "fluent-bit-config", nil // Stub - use your actual implementation
}

func getSecretByLabel(ctx context.Context, c client.Client, namespace, key, value, controllerNS string) (string, error) {
	// Implementation from your helpers.go - simplified for brevity
	return "loki-credentials", nil // Stub - use your actual implementation
}
