package jobs

import (
	"context"
	"fmt"
	"strings"
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

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	"github.com/kentrasecurity/kentra/internal/controller/config"
	"github.com/kentrasecurity/kentra/internal/controller/pods"
)

// AttackSpec contains all fields needed to build a job
type AttackSpec struct {
	// Common fields (all attacks)
	Tool          string
	Targets       []string
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

	// Exploit-specific fields
	Module       string
	Payload      string
	RawCommand   string
	Capabilities []string
	ReverseShell *securityv1alpha1.ReverseShellConfig
}

// AttackStatus contains status fields
type AttackStatus struct {
	State        string
	LastExecuted string
	JobName      string
}

// JobBuilder builds and manages jobs/cronjobs
type JobBuilder struct {
	Client              client.Client
	Scheme              *runtime.Scheme
	Configurator        *config.ToolsConfigurator
	ControllerNamespace string
	ResourceType        string
}

// ReconcileJob creates or updates a single job
func (jb *JobBuilder) ReconcileJob(
	ctx context.Context,
	owner client.Object,
	jobName string,
	spec *AttackSpec,
	updateStatus func(*AttackStatus),
) (ctrl.Result, error) {
	if spec.Periodic {
		return jb.reconcileCronJob(ctx, owner, jobName, spec, updateStatus)
	}
	return jb.reconcileOneTimeJob(ctx, owner, jobName, spec, updateStatus)
}

func (jb *JobBuilder) reconcileOneTimeJob(
	ctx context.Context,
	owner client.Object,
	jobName string,
	spec *AttackSpec,
	updateStatus func(*AttackStatus),
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	job := &batchv1.Job{}
	err := jb.Client.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: owner.GetNamespace(),
	}, job)

	if err != nil && errors.IsNotFound(err) {
		// Create new job
		newJob, err := jb.buildJob(ctx, owner, jobName, spec)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := jb.Client.Create(ctx, newJob); err != nil {
			log.Error(err, "Failed to create Job")
			return ctrl.Result{}, err
		}

		updateStatus(&AttackStatus{
			State:        "Running",
			LastExecuted: time.Now().Format(time.RFC3339),
			JobName:      jobName,
		})

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	// Job exists - check if needs recreation
	currentGen := fmt.Sprintf("%d", owner.GetGeneration())
	existingGen := job.Annotations["kentra.sh/parent-generation"]

	if existingGen != currentGen {
		log.Info("Resource changed, deleting and recreating job")
		if err := jb.Client.Delete(ctx, job); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (jb *JobBuilder) reconcileCronJob(
	ctx context.Context,
	owner client.Object,
	jobName string,
	spec *AttackSpec,
	updateStatus func(*AttackStatus),
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cronJob := &batchv1.CronJob{}
	err := jb.Client.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: owner.GetNamespace(),
	}, cronJob)

	if err != nil && errors.IsNotFound(err) {
		// Create new cronjob
		newCronJob, err := jb.buildCronJob(ctx, owner, jobName, spec)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := jb.Client.Create(ctx, newCronJob); err != nil {
			log.Error(err, "Failed to create CronJob")
			return ctrl.Result{}, err
		}

		updateStatus(&AttackStatus{
			State:        "Scheduled",
			LastExecuted: time.Now().Format(time.RFC3339),
			JobName:      jobName,
		})

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	// CronJob exists - check if needs recreation
	currentGen := fmt.Sprintf("%d", owner.GetGeneration())
	existingGen := cronJob.Annotations["kentra.sh/parent-generation"]

	if existingGen != currentGen {
		log.Info("Resource changed, deleting and recreating cronjob")
		if err := jb.Client.Delete(ctx, cronJob); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (jb *JobBuilder) buildJob(
	ctx context.Context,
	owner client.Object,
	jobName string,
	spec *AttackSpec,
) (*batchv1.Job, error) {
	// Get separator from tool config
	toolConfig, err := jb.Configurator.GetToolConfig(spec.Tool)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool config: %w", err)
	}

	separator := toolConfig.Separator
	if separator == "" {
		separator = " " // Default to space
	}

	// Join targets with the tool-specific separator
	target := strings.Join(spec.Targets, separator)

	podSpec, err := pods.BuildPodSpec(ctx, jb.Client, spec.Tool, target, spec.Port,
		spec.Args, spec.HTTPProxy, spec.AdditionalEnv, spec.Debug, spec.Files, spec.Assets,
		jb.Configurator, owner.GetNamespace(), jobName, jb.ResourceType, jb.ControllerNamespace)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"app":                     jb.ResourceType,
		"tool":                    spec.Tool,
		"kentra.sh/resource-type": "attack",
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: owner.GetNamespace(),
			Labels:    labels,
			Annotations: map[string]string{
				"kentra.sh/target":            target,
				"kentra.sh/tool":              spec.Tool,
				"kentra.sh/category":          spec.Category,
				"kentra.sh/parent-generation": fmt.Sprintf("%d", owner.GetGeneration()),
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

	if err := controllerutil.SetControllerReference(owner, job, jb.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func (jb *JobBuilder) buildCronJob(
	ctx context.Context,
	owner client.Object,
	jobName string,
	spec *AttackSpec,
) (*batchv1.CronJob, error) {
	// Get separator from tool config
	toolConfig, err := jb.Configurator.GetToolConfig(spec.Tool)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool config: %w", err)
	}

	separator := toolConfig.Separator
	if separator == "" {
		separator = " " // Default to space
	}

	// Join targets with the tool-specific separator
	target := strings.Join(spec.Targets, separator)

	podSpec, err := pods.BuildPodSpec(ctx, jb.Client, spec.Tool, target, spec.Port,
		spec.Args, spec.HTTPProxy, spec.AdditionalEnv, spec.Debug, spec.Files, spec.Assets,
		jb.Configurator, owner.GetNamespace(), jobName, jb.ResourceType, jb.ControllerNamespace)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"app":                     jb.ResourceType,
		"tool":                    spec.Tool,
		"kentra.sh/resource-type": "attack",
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: owner.GetNamespace(),
			Labels:    labels,
			Annotations: map[string]string{
				"kentra.sh/target":            target,
				"kentra.sh/tool":              spec.Tool,
				"kentra.sh/category":          spec.Category,
				"kentra.sh/parent-generation": fmt.Sprintf("%d", owner.GetGeneration()),
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

	if err := controllerutil.SetControllerReference(owner, cronJob, jb.Scheme); err != nil {
		return nil, err
	}

	return cronJob, nil
}

func int32Ptr(i int32) *int32 {
	return &i
}
