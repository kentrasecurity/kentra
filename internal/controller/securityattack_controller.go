package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kttack/kttack/api/v1alpha1"
)

// SecurityAttackReconciler reconciles a SecurityAttack object
type SecurityAttackReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ToolSpecManager *ToolSpecManager
	Configurator    *ToolsConfigurator
}

//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kttack.io,resources=securityattacks/finalizers,verbs=update
//+kubebuilder:rbac:groups=kttack.io,resources=targetpools,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;configmaps,verbs=get;list;watch

func (r *SecurityAttackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Load tool configurations from ConfigMap if not already loaded
	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool specifications ConfigMap - controller cannot proceed", "ConfigMap", "kttack-tool-specs")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	sa := &securityv1alpha1.SecurityAttack{}
	if err := r.Get(ctx, req.NamespacedName, sa); err != nil {
		if errors.IsNotFound(err) {
			log.Info("SecurityAttack resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get SecurityAttack")
		return ctrl.Result{}, err
	}

	// Ensure labels are set
	if sa.Labels == nil {
		sa.Labels = make(map[string]string)
	}
	needsUpdate := false
	if sa.Labels["kttack-resource-type"] != "attack" {
		sa.Labels["kttack-resource-type"] = "attack"
		needsUpdate = true
	}

	// Update the resource if labels were modified
	if needsUpdate {
		if err := r.Update(ctx, sa); err != nil {
			log.Error(err, "Failed to update SecurityAttack labels")
			return ctrl.Result{}, err
		}
	}

	// Resolve TargetPool reference if provided
	if sa.Spec.TargetPool != "" {
		tg := &securityv1alpha1.TargetPool{}
		tgNN := types.NamespacedName{Name: sa.Spec.TargetPool, Namespace: sa.Namespace}
		if err := r.Get(ctx, tgNN, tg); err != nil {
			log.Error(err, "Failed to get referenced TargetPool", "TargetPool", sa.Spec.TargetPool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		// Set target and port from TargetPool
		sa.Spec.Target = tg.Spec.Target
		// Update resolved status
		sa.Status.ResolvedTarget = tg.Spec.Target
	} else {
		// Use direct target
		sa.Status.ResolvedTarget = sa.Spec.Target
	}

	// Validate that either Target or TargetPool is set
	if sa.Spec.Target == "" {
		log.Error(fmt.Errorf("neither target nor targetPool specified"), "Invalid SecurityAttack resource")
		return ctrl.Result{}, fmt.Errorf("SecurityAttack must have either 'target' or 'targetPool' specified")
	}

	targetNamespace := sa.Namespace
	jobName := sa.Name
	cronJobName := fmt.Sprintf("%s-cronjob", sa.Name)

	// Convert to SecurityResource adapter
	adapter := &securityAttackAdapter{sa}

	// Check if we need to create a job or cronjob
	if sa.Spec.Periodic {
		// Create or update CronJob
		cronJob := &batchv1.CronJob{}
		cronJobNN := types.NamespacedName{Name: cronJobName, Namespace: targetNamespace}

		err := r.Get(ctx, cronJobNN, cronJob)
		if err != nil && errors.IsNotFound(err) {
			// Create new CronJob
			cronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, cronJobName, targetNamespace, "security-attack")
			if err != nil {
				log.Error(err, "Failed to build CronJob")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "Failed to create CronJob")
				return ctrl.Result{}, err
			}
			log.Info("Created new CronJob", "CronJob", cronJobName)
			sa.Status.State = "Running"
			sa.Status.LastExecuted = time.Now().Format(time.RFC3339)
			if err := r.Status().Update(ctx, sa); err != nil {
				log.Error(err, "Failed to update status")
			}
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
			job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, jobName, targetNamespace, "security-attack")
			if err != nil {
				log.Error(err, "Failed to build Job")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Failed to create Job")
				return ctrl.Result{}, err
			}
			log.Info("Created new Job", "Job", jobName)
			sa.Status.State = "Running"
			sa.Status.LastExecuted = time.Now().Format(time.RFC3339)
			if err := r.Status().Update(ctx, sa); err != nil {
				log.Error(err, "Failed to update status")
			}
		} else if err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		}
	}

	// Update status with resolved target
	if err := r.Status().Update(ctx, sa); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// securityAttackAdapter adapts SecurityAttack to SecurityResource interface
type securityAttackAdapter struct {
	sa *securityv1alpha1.SecurityAttack
}

func (a *securityAttackAdapter) GetName() string {
	return a.sa.Name
}

func (a *securityAttackAdapter) GetNamespace() string {
	return a.sa.Namespace
}

func (a *securityAttackAdapter) GetSpec() *ResourceSpec {
	envVars := make([]corev1.EnvVar, len(a.sa.Spec.AdditionalEnv))
	for i, ev := range a.sa.Spec.AdditionalEnv {
		envVars[i] = corev1.EnvVar{
			Name:  ev.Name,
			Value: ev.Value,
		}
	}
	return &ResourceSpec{
		Tool:          a.sa.Spec.Tool,
		Target:        a.sa.Spec.Target,
		Category:      a.sa.Spec.Category,
		Args:          a.sa.Spec.Args,
		HTTPProxy:     a.sa.Spec.HTTPProxy,
		AdditionalEnv: envVars,
		Debug:         a.sa.Spec.Debug,
		Periodic:      a.sa.Spec.Periodic,
		Schedule:      a.sa.Spec.Schedule,
		Files:         []string{},
	}
}

func (a *securityAttackAdapter) GetStatus() *ResourceStatus {
	return &ResourceStatus{
		State:        a.sa.Status.State,
		LastExecuted: a.sa.Status.LastExecuted,
	}
}

func (a *securityAttackAdapter) GetKubeObject() client.Object {
	return a.sa
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecurityAttackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.SecurityAttack{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
