package controller

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

	securityv1alpha1 "github.com/yourorg/security-operator/api/v1alpha1"
)

// SecurityAttackReconciler reconciles a SecurityAttack object
type SecurityAttackReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ToolSpecManager *ToolSpecManager
}

//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps,verbs=get;list;watch

func (r *SecurityAttackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	sa := &securityv1alpha1.SecurityAttack{}
	if err := r.Get(ctx, req.NamespacedName, sa); err != nil {
		if errors.IsNotFound(err) {
			log.Info("SecurityAttack resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get SecurityAttack")
		return ctrl.Result{}, err
	}

	if err := r.ToolSpecManager.LoadToolSpecs(ctx); err != nil {
		log.Error(err, "Failed to load tool specifications")
		return ctrl.Result{}, err
	}

	if sa.Spec.Periodic {
		return r.reconcileCronJob(ctx, sa)
	}
	return r.reconcileJob(ctx, sa)
}

// ----------------------------
// Job Handling
// ----------------------------
func (r *SecurityAttackReconciler) reconcileJob(ctx context.Context, sa *securityv1alpha1.SecurityAttack) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	jobName := fmt.Sprintf("%s-job", sa.Name)
	jobNamespace := sa.Spec.TargetNamespace
	if jobNamespace == "" {
		jobNamespace = sa.Namespace
	}

	found := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: jobNamespace}, found)
	if err != nil && errors.IsNotFound(err) {
		job, err := r.buildJob(sa, jobName, jobNamespace)
		if err != nil {
			log.Error(err, "Failed to build Job")
			r.updateStatus(ctx, sa, "Failed")
			return ctrl.Result{}, err
		}

		if err := controllerutil.SetControllerReference(sa, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Creating new Job", "Namespace", job.Namespace, "Name", job.Name)
		if err := r.Create(ctx, job); err != nil {
			log.Error(err, "Failed to create Job")
			return ctrl.Result{}, err
		}

		sa.Status.JobName = jobName
		sa.Status.State = "Running"
		sa.Status.LastExecuted = metav1.Now().Format(time.RFC3339)
		if err := r.Status().Update(ctx, sa); err != nil {
			log.Error(err, "Failed to update SecurityAttack status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if found.Status.Succeeded > 0 {
		r.updateStatus(ctx, sa, "Completed")
	} else if found.Status.Failed > 0 {
		r.updateStatus(ctx, sa, "Failed")
	}

	log.Info("Job already exists", "Namespace", found.Namespace, "Name", found.Name)
	return ctrl.Result{}, nil
}

// ----------------------------
// CronJob Handling
// ----------------------------
func (r *SecurityAttackReconciler) reconcileCronJob(ctx context.Context, sa *securityv1alpha1.SecurityAttack) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cronJobName := fmt.Sprintf("%s-cronjob", sa.Name)
	cronJobNamespace := sa.Spec.TargetNamespace
	if cronJobNamespace == "" {
		cronJobNamespace = sa.Namespace
	}

	found := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{Name: cronJobName, Namespace: cronJobNamespace}, found)
	if err != nil && errors.IsNotFound(err) {
		cronJob, err := r.buildCronJob(sa, cronJobName, cronJobNamespace)
		if err != nil {
			log.Error(err, "Failed to build CronJob")
			return ctrl.Result{}, err
		}

		if err := controllerutil.SetControllerReference(sa, cronJob, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Creating new CronJob", "Namespace", cronJob.Namespace, "Name", cronJob.Name)
		if err := r.Create(ctx, cronJob); err != nil {
			log.Error(err, "Failed to create CronJob")
			return ctrl.Result{}, err
		}

		sa.Status.JobName = cronJobName
		sa.Status.State = "Running"
		if err := r.Status().Update(ctx, sa); err != nil {
			log.Error(err, "Failed to update SecurityAttack status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if found.Spec.Schedule != sa.Spec.Schedule {
		found.Spec.Schedule = sa.Spec.Schedule
		log.Info("Updating CronJob schedule", "NewSchedule", sa.Spec.Schedule)
		if err := r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update CronJob")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// ----------------------------
// Builders
// ----------------------------
func (r *SecurityAttackReconciler) buildJob(sa *securityv1alpha1.SecurityAttack, name, namespace string) (*batchv1.Job, error) {
	labels := map[string]string{
		"app":         "security-attack",
		"attack-type": sa.Spec.AttackType,
		"controller":  sa.Name,
		"tool":        sa.Spec.Tool,
	}

	podSpec, err := r.buildPodSpec(sa)
	if err != nil {
		return nil, err
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kttack.io/target":      sa.Spec.Target,
				"kttack.io/attack-type": sa.Spec.AttackType,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(2),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
		},
	}, nil
}

func (r *SecurityAttackReconciler) buildCronJob(sa *securityv1alpha1.SecurityAttack, name, namespace string) (*batchv1.CronJob, error) {
	labels := map[string]string{
		"app":         "security-attack",
		"attack-type": sa.Spec.AttackType,
		"controller":  sa.Name,
		"tool":        sa.Spec.Tool,
	}

	podSpec, err := r.buildPodSpec(sa)
	if err != nil {
		return nil, err
	}

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kttack.io/target":      sa.Spec.Target,
				"kttack.io/attack-type": sa.Spec.AttackType,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: sa.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit: int32Ptr(2),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec:       podSpec,
					},
				},
			},
		},
	}, nil
}

func (r *SecurityAttackReconciler) buildPodSpec(sa *securityv1alpha1.SecurityAttack) (corev1.PodSpec, error) {
	image, err := r.ToolSpecManager.GetToolImage(sa.Spec.Tool)
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("failed to get tool image: %w", err)
	}

	command, err := r.ToolSpecManager.BuildCommand(sa.Spec.Tool, sa.Spec.Target, sa.Spec.Args)
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("failed to build command: %w", err)
	}

	// Wrap command in shell to properly handle redirection
	shellCommand := joinCommand(command)
	logsArgs := []string{
		"sh",
		"-c",
		fmt.Sprintf("%s > /logs/output.log 2>&1", shellCommand),
	}

	container := corev1.Container{
		Name:  "security-tool",
		Image: image,
		Args:  logsArgs, // redirect logs to shared emptyDir via shell
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "logs",
				MountPath: "/logs",
			},
		},
	}

	// Fluent-bit sidecar container
	lokiLabels := fmt.Sprintf("app=security-attack,attack_type=%s,tool=%s,namespace=$(NAMESPACE),job=$(JOB_NAME)", sa.Spec.AttackType, sa.Spec.Tool)
	fluentBitContainer := corev1.Container{
		Name:  "fluent-bit",
		Image: "fluent/fluent-bit:latest",
		Args: []string{
			"-i", "tail",
			"-p", "Path=/logs/*.log",
			"-p", "Read_from_Head=true",
			"-p", "Refresh_Interval=5",
			"-p", "Tag=security-attack",
			"-o", "loki",
			"-p", "host=192.168.1.172",
			"-p", "port=3100",
			"-p", "tls=off",
			"-p", "tls.verify=off",
			"-p", fmt.Sprintf("labels=%s", lokiLabels),
			"-p", "tenant_id=1",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "logs",
				MountPath: "/logs",
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "JOB_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
	}

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers:    []corev1.Container{container, fluentBitContainer},
		Volumes: []corev1.Volume{
			{
				Name: "logs",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}

	capabilities, err := r.ToolSpecManager.GetToolCapabilities(sa.Spec.Tool)
	if err == nil && capabilities != nil && len(capabilities.Add) > 0 {
		caps := make([]corev1.Capability, len(capabilities.Add))
		for i, cap := range capabilities.Add {
			caps[i] = corev1.Capability(cap)
		}
		container.SecurityContext = &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: caps,
			},
		}
		podSpec.Containers[0] = container
	}

	return podSpec, nil
}

// ----------------------------
// Helpers
// ----------------------------
func (r *SecurityAttackReconciler) updateStatus(ctx context.Context, sa *securityv1alpha1.SecurityAttack, state string) {
	sa.Status.State = state
	if err := r.Status().Update(ctx, sa); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

func (r *SecurityAttackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.SecurityAttack{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}

func int32Ptr(i int32) *int32 { return &i }

func joinCommand(cmd []string) string {
	result := ""
	for i, arg := range cmd {
		if i > 0 {
			result += " "
		}
		if strings.Contains(arg, " ") {
			result += fmt.Sprintf("\"%s\"", arg)
		} else {
			result += arg
		}
	}
	return result
}
