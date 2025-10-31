package controller

import (
	"context"
	"fmt"
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

// CustomAttackReconciler reconciles a CustomAttack object
type CustomAttackReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kttack.io,resources=customattacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=customattacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=customattacks/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

func (r *CustomAttackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the CustomAttack instance
	customAttack := &securityv1alpha1.CustomAttack{}
	err := r.Get(ctx, req.NamespacedName, customAttack)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("CustomAttack resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get CustomAttack")
		return ctrl.Result{}, err
	}

	// Check if periodic execution is needed
	if customAttack.Spec.Periodic {
		return r.reconcileCronJob(ctx, customAttack)
	}

	return r.reconcileJob(ctx, customAttack)
}

func (r *CustomAttackReconciler) reconcileJob(ctx context.Context, ca *securityv1alpha1.CustomAttack) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Job is created in the same namespace as the CustomAttack CR
	jobName := fmt.Sprintf("%s-job", ca.Name)
	found := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: ca.Namespace, // Same namespace as CR
	}, found)

	if err != nil && errors.IsNotFound(err) {
		// Create new job
		job := r.buildJob(ca, jobName)
		log.Info("Creating a new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)

		if err := r.Create(ctx, job); err != nil {
			log.Error(err, "Failed to create new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			r.updateStatus(ctx, ca, "Failed", fmt.Sprintf("Failed to create job: %v", err))
			return ctrl.Result{}, err
		}

		// Update status
		r.updateStatus(ctx, ca, "Running", fmt.Sprintf("Job %s created", jobName))
		ca.Status.JobName = jobName
		ca.Status.LastExecuted = metav1.Now().Format(time.RFC3339)
		if err := r.Status().Update(ctx, ca); err != nil {
			log.Error(err, "Failed to update CustomAttack status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Job")
		return ctrl.Result{}, err
	}

	// Job already exists - check status
	if found.Status.Succeeded > 0 {
		r.updateStatus(ctx, ca, "Completed", "Job completed successfully")
	} else if found.Status.Failed > 0 {
		r.updateStatus(ctx, ca, "Failed", "Job failed")
	}

	log.Info("Job already exists", "Job.Namespace", found.Namespace, "Job.Name", found.Name)
	return ctrl.Result{}, nil
}

func (r *CustomAttackReconciler) reconcileCronJob(ctx context.Context, ca *securityv1alpha1.CustomAttack) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cronJobName := fmt.Sprintf("%s-cronjob", ca.Name)
	found := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      cronJobName,
		Namespace: ca.Namespace, // Same namespace as CR
	}, found)

	if err != nil && errors.IsNotFound(err) {
		// Create new cronjob
		cronJob := r.buildCronJob(ca, cronJobName)
		log.Info("Creating a new CronJob", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)

		if err := r.Create(ctx, cronJob); err != nil {
			log.Error(err, "Failed to create new CronJob")
			r.updateStatus(ctx, ca, "Failed", fmt.Sprintf("Failed to create cronjob: %v", err))
			return ctrl.Result{}, err
		}

		// Update status
		r.updateStatus(ctx, ca, "Running", fmt.Sprintf("CronJob %s created with schedule: %s", cronJobName, ca.Spec.Schedule))
		ca.Status.JobName = cronJobName
		if err := r.Status().Update(ctx, ca); err != nil {
			log.Error(err, "Failed to update CustomAttack status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get CronJob")
		return ctrl.Result{}, err
	}

	// CronJob exists, check if update is needed
	needsUpdate := false

	if found.Spec.Schedule != ca.Spec.Schedule {
		found.Spec.Schedule = ca.Spec.Schedule
		needsUpdate = true
		log.Info("Schedule changed", "old", found.Spec.Schedule, "new", ca.Spec.Schedule)
	}

	// Check if image changed
	if len(found.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
		if found.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image != ca.Spec.Tool {
			found.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = ca.Spec.Tool
			needsUpdate = true
			log.Info("Image changed", "new", ca.Spec.Tool)
		}
	}

	if needsUpdate {
		if err := r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update CronJob")
			return ctrl.Result{}, err
		}
		log.Info("Updated CronJob")
		r.updateStatus(ctx, ca, "Running", "CronJob updated")
	}

	return ctrl.Result{}, nil
}

func (r *CustomAttackReconciler) buildJob(ca *securityv1alpha1.CustomAttack, jobName string) *batchv1.Job {
	labels := map[string]string{
		"app":                   "custom-attack",
		"controller":            ca.Name,
		"kttack.io/cr": ca.Name,
	}

	podSpec := r.buildPodSpec(ca, labels)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ca.Namespace, // Same namespace as CR
			Labels:    labels,
			Annotations: map[string]string{
				"kttack.io/description": ca.Spec.Description,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
			BackoffLimit: int32Ptr(2),
		},
	}

	// Set owner reference
	controllerutil.SetControllerReference(ca, job, r.Scheme)
	return job
}

func (r *CustomAttackReconciler) buildCronJob(ca *securityv1alpha1.CustomAttack, cronJobName string) *batchv1.CronJob {
	labels := map[string]string{
		"app":                   "custom-attack",
		"controller":            ca.Name,
		"kttack.io/cr": ca.Name,
	}

	podSpec := r.buildPodSpec(ca, labels)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: ca.Namespace, // Same namespace as CR
			Labels:    labels,
			Annotations: map[string]string{
				"kttack.io/description": ca.Spec.Description,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: ca.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: podSpec,
					},
					BackoffLimit: int32Ptr(2),
				},
			},
		},
	}

	// Set owner reference
	controllerutil.SetControllerReference(ca, cronJob, r.Scheme)
	return cronJob
}

func (r *CustomAttackReconciler) buildPodSpec(ca *securityv1alpha1.CustomAttack, labels map[string]string) corev1.PodSpec {
	container := corev1.Container{
		Name:            "custom-tool",
		Image:           ca.Spec.Tool,
		ImagePullPolicy: ca.Spec.ImagePullPolicy,
	}

	// Set command if provided
	if len(ca.Spec.Command) > 0 {
		container.Command = ca.Spec.Command
	}

	// Set args
	if len(ca.Spec.Args) > 0 {
		container.Args = ca.Spec.Args
	}

	// Add environment variables
	if len(ca.Spec.EnvVars) > 0 {
		envVars := make([]corev1.EnvVar, len(ca.Spec.EnvVars))
		for i, ev := range ca.Spec.EnvVars {
			envVars[i] = corev1.EnvVar{
				Name:  ev.Name,
				Value: ev.Value,
			}
		}
		container.Env = envVars
	}

	// Set resource requirements
	if ca.Spec.Resources != nil {
		container.Resources = *ca.Spec.Resources
	} else {
		// Set default resource limits
		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		}
	}

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers:    []corev1.Container{container},
	}

	// Add image pull secrets if specified
	if len(ca.Spec.ImagePullSecrets) > 0 {
		secrets := make([]corev1.LocalObjectReference, len(ca.Spec.ImagePullSecrets))
		for i, secretName := range ca.Spec.ImagePullSecrets {
			secrets[i] = corev1.LocalObjectReference{Name: secretName}
		}
		podSpec.ImagePullSecrets = secrets
	}

	// Set security context
	if ca.Spec.SecurityContext != nil {
		podSecurityContext := &corev1.PodSecurityContext{}

		if ca.Spec.SecurityContext.RunAsUser != nil {
			podSecurityContext.RunAsUser = ca.Spec.SecurityContext.RunAsUser
		}
		if ca.Spec.SecurityContext.RunAsGroup != nil {
			podSecurityContext.RunAsGroup = ca.Spec.SecurityContext.RunAsGroup
		}
		if ca.Spec.SecurityContext.FSGroup != nil {
			podSecurityContext.FSGroup = ca.Spec.SecurityContext.FSGroup
		}
		if ca.Spec.SecurityContext.RunAsNonRoot != nil {
			podSecurityContext.RunAsNonRoot = ca.Spec.SecurityContext.RunAsNonRoot
		}

		podSpec.SecurityContext = podSecurityContext

		// Set container security context for capabilities
		if ca.Spec.SecurityContext.Capabilities != nil {
			containerSecurityContext := &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{},
			}

			if len(ca.Spec.SecurityContext.Capabilities.Add) > 0 {
				caps := make([]corev1.Capability, len(ca.Spec.SecurityContext.Capabilities.Add))
				for i, cap := range ca.Spec.SecurityContext.Capabilities.Add {
					caps[i] = corev1.Capability(cap)
				}
				containerSecurityContext.Capabilities.Add = caps
			}

			if len(ca.Spec.SecurityContext.Capabilities.Drop) > 0 {
				caps := make([]corev1.Capability, len(ca.Spec.SecurityContext.Capabilities.Drop))
				for i, cap := range ca.Spec.SecurityContext.Capabilities.Drop {
					caps[i] = corev1.Capability(cap)
				}
				containerSecurityContext.Capabilities.Drop = caps
			}

			podSpec.Containers[0].SecurityContext = containerSecurityContext
		}
	}

	return podSpec
}

func (r *CustomAttackReconciler) updateStatus(ctx context.Context, ca *securityv1alpha1.CustomAttack, state, message string) {
	ca.Status.State = state
	ca.Status.Message = message
	if err := r.Status().Update(ctx, ca); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

func (r *CustomAttackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.CustomAttack{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
