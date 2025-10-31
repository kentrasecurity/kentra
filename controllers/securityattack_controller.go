package controllers

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

// SecurityAttackReconciler reconciles a SecurityAttack object
type SecurityAttackReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

func (r *SecurityAttackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the SecurityAttack instance
	securityAttack := &securityv1alpha1.SecurityAttack{}
	err := r.Get(ctx, req.NamespacedName, securityAttack)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("SecurityAttack resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get SecurityAttack")
		return ctrl.Result{}, err
	}

	// Check if periodic execution is needed
	if securityAttack.Spec.Periodic {
		return r.reconcileCronJob(ctx, securityAttack)
	}

	return r.reconcileJob(ctx, securityAttack)
}

func (r *SecurityAttackReconciler) reconcileJob(ctx context.Context, sa *securityv1alpha1.SecurityAttack) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check if job already exists
	jobName := fmt.Sprintf("%s-%s", sa.Name, "job")
	found := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: sa.Spec.TargetNamespace,
	}, found)

	if err != nil && errors.IsNotFound(err) {
		// Create new job
		job := r.buildJob(sa, jobName)
		log.Info("Creating a new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)

		if err := r.Create(ctx, job); err != nil {
			log.Error(err, "Failed to create new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			return ctrl.Result{}, err
		}

		// Update status
		sa.Status.JobName = jobName
		sa.Status.State = "Running"
		sa.Status.LastExecuted = metav1.Now().Format(time.RFC3339)
		if err := r.Status().Update(ctx, sa); err != nil {
			log.Error(err, "Failed to update SecurityAttack status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Job")
		return ctrl.Result{}, err
	}

	// Job already exists
	log.Info("Job already exists", "Job.Namespace", found.Namespace, "Job.Name", found.Name)
	return ctrl.Result{}, nil
}

func (r *SecurityAttackReconciler) reconcileCronJob(ctx context.Context, sa *securityv1alpha1.SecurityAttack) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cronJobName := fmt.Sprintf("%s-%s", sa.Name, "cronjob")
	found := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      cronJobName,
		Namespace: sa.Spec.TargetNamespace,
	}, found)

	if err != nil && errors.IsNotFound(err) {
		// Create new cronjob
		cronJob := r.buildCronJob(sa, cronJobName)
		log.Info("Creating a new CronJob", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)

		if err := r.Create(ctx, cronJob); err != nil {
			log.Error(err, "Failed to create new CronJob")
			return ctrl.Result{}, err
		}

		// Update status
		sa.Status.JobName = cronJobName
		sa.Status.State = "Running"
		if err := r.Status().Update(ctx, sa); err != nil {
			log.Error(err, "Failed to update SecurityAttack status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get CronJob")
		return ctrl.Result{}, err
	}

	// CronJob exists, check if update is needed
	if found.Spec.Schedule != sa.Spec.Schedule {
		found.Spec.Schedule = sa.Spec.Schedule
		if err := r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update CronJob")
			return ctrl.Result{}, err
		}
		log.Info("Updated CronJob schedule")
	}

	return ctrl.Result{}, nil
}

func (r *SecurityAttackReconciler) buildJob(sa *securityv1alpha1.SecurityAttack, jobName string) *batchv1.Job {
	labels := map[string]string{
		"app":         "security-attack",
		"attack-type": sa.Spec.AttackType,
		"controller":  sa.Name,
	}

	// Build command based on tool
	command := r.buildCommand(sa)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: sa.Spec.TargetNamespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "security-tool",
							Image:   r.getToolImage(sa.Spec.Tool),
							Command: command,
						},
					},
				},
			},
			BackoffLimit: int32Ptr(2),
		},
	}

	// Set owner reference
	controllerutil.SetControllerReference(sa, job, r.Scheme)
	return job
}

func (r *SecurityAttackReconciler) buildCronJob(sa *securityv1alpha1.SecurityAttack, cronJobName string) *batchv1.CronJob {
	labels := map[string]string{
		"app":         "security-attack",
		"attack-type": sa.Spec.AttackType,
		"controller":  sa.Name,
	}

	command := r.buildCommand(sa)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: sa.Spec.TargetNamespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: sa.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:    "security-tool",
									Image:   r.getToolImage(sa.Spec.Tool),
									Command: command,
								},
							},
						},
					},
					BackoffLimit: int32Ptr(2),
				},
			},
		},
	}

	// Set owner reference
	controllerutil.SetControllerReference(sa, cronJob, r.Scheme)
	return cronJob
}

func (r *SecurityAttackReconciler) buildCommand(sa *securityv1alpha1.SecurityAttack) []string {
	baseCommand := []string{}

	switch sa.Spec.Tool {
	case "nmap":
		baseCommand = []string{"nmap", "-v", sa.Spec.Target}
	case "nikto":
		baseCommand = []string{"nikto", "-h", sa.Spec.Target}
	case "sqlmap":
		baseCommand = []string{"sqlmap", "-u", sa.Spec.Target}
	default:
		baseCommand = []string{sa.Spec.Tool, sa.Spec.Target}
	}

	// Append additional args if provided
	if len(sa.Spec.Args) > 0 {
		baseCommand = append(baseCommand, sa.Spec.Args...)
	}

	return baseCommand
}

func (r *SecurityAttackReconciler) getToolImage(tool string) string {
	// Map tools to container images
	imageMap := map[string]string{
		"nmap":   "instrumentisto/nmap:latest",
		"nikto":  "sullo/nikto:latest",
		"sqlmap": "paoloo/sqlmap:latest",
	}

	if image, ok := imageMap[tool]; ok {
		return image
	}

	// Default to a generic security tools image
	return fmt.Sprintf("security/%s:latest", tool)
}

func (r *SecurityAttackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.SecurityAttack{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}

func int32Ptr(i int32) *int32 {
	return &i
}
