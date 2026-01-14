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

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

type OsintReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *ToolsConfigurator
	ControllerNamespace string
}

// +kubebuilder:rbac:groups=kentra.sh,resources=osints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kentra.sh,resources=osints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kentra.sh,resources=osints/finalizers,verbs=update
// +kubebuilder:rbac:groups=kentra.sh,resources=targetpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=kentra.sh,resources=assetpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=kentra.sh,resources=storagepools,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list

func (r *OsintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool configurations, retrying...")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	osint := &securityv1alpha1.Osint{}
	if err := r.Get(ctx, req.NamespacedName, osint); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	isManaged, err := isNamespaceManagedByKentra(ctx, r.Client, osint.Namespace)
	if err != nil || !isManaged {
		return ctrl.Result{}, fmt.Errorf("namespace %s is not managed by Kentra", osint.Namespace)
	}

	// Label management
	if osint.Labels == nil {
		osint.Labels = make(map[string]string)
	}
	if osint.Labels["kentra.sh/resource-type"] != "attack" {
		osint.Labels["kentra.sh/resource-type"] = "attack"
		if err := r.Update(ctx, osint); err != nil {
			return ctrl.Result{}, err
		}
	}

	var resolvedFiles []string
	if osint.Spec.StoragePool != "" {
		sg := &securityv1alpha1.StoragePool{}
		if err := r.Get(ctx, types.NamespacedName{Name: osint.Spec.StoragePool, Namespace: osint.Namespace}, sg); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		resolvedFiles = sg.Spec.Files
	}

	if osint.Spec.AssetPool != "" {
		ap := &securityv1alpha1.AssetPool{}
		if err := r.Get(ctx, types.NamespacedName{Name: osint.Spec.AssetPool, Namespace: osint.Namespace}, ap); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		// Updated logic: if pool has items, we reconcile multiple jobs (one per group/person)
		if len(ap.Spec.Pool) > 0 {
			return r.reconcileMultipleJobs(ctx, osint, ap, resolvedFiles)
		}
		return ctrl.Result{}, fmt.Errorf("assetPool %s has no pool items", osint.Spec.AssetPool)
	}

	if osint.Spec.TargetPool != "" {
		tg := &securityv1alpha1.TargetPool{}
		if err := r.Get(ctx, types.NamespacedName{Name: osint.Spec.TargetPool, Namespace: osint.Namespace}, tg); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		osint.Spec.Target = tg.Spec.Target
		osint.Status.ResolvedTarget = tg.Spec.Target
	} else {
		osint.Status.ResolvedTarget = osint.Spec.Target
	}

	if osint.Spec.Target == "" {
		return ctrl.Result{}, fmt.Errorf("no target specified")
	}

	return r.reconcileSingleJobWithTarget(ctx, osint, resolvedFiles)
}

func (r *OsintReconciler) reconcileMultipleJobs(ctx context.Context, osint *securityv1alpha1.Osint, ap *securityv1alpha1.AssetPool, resolvedFiles []string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	createdJobs, existingJobs := 0, 0

	// Iterate over the corrected .Spec.Pool field
	for i, item := range ap.Spec.Pool {
		if len(item.Assets) == 0 {
			continue
		}

		// Generate job name based on the person's name (e.g., osintname-john)
		jobName := generateJobName(osint.Name, item.Name, i)

		if err := r.createJobForGroup(ctx, osint, jobName, resolvedFiles, item.Assets, &createdJobs, &existingJobs); err != nil {
			log.Error(err, "Failed to create job for group", "group", item.Name)
			continue
		}
	}

	// Update Status
	osint.Status.State = "Running"
	osint.Status.JobName = fmt.Sprintf("%d jobs managed", createdJobs+existingJobs)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, r.Status().Update(ctx, osint)
}

func (r *OsintReconciler) createJobForGroup(
	ctx context.Context,
	osint *securityv1alpha1.Osint,
	jobName string,
	resolvedFiles []string,
	assets []securityv1alpha1.AssetItem,
	createdJobs *int,
	existingJobs *int,
) error {
	adapter := &OsintAdapter{
		osint:          osint,
		resolvedFiles:  resolvedFiles,
		resolvedAssets: assets,
	}

	targetNamespace := osint.Namespace

	if osint.Spec.Periodic {
		cronJob := &batchv1.CronJob{}
		err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: targetNamespace}, cronJob)
		if err != nil && errors.IsNotFound(err) {
			newCJ, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, jobName, targetNamespace, "osint", r.ControllerNamespace)
			if err != nil {
				return err
			}
			if err := r.Create(ctx, newCJ); err != nil {
				return err
			}
			*createdJobs++
		} else if err == nil {
			if cronJob.Annotations["kentra.sh/parent-generation"] != fmt.Sprintf("%d", osint.Generation) {
				_ = r.Delete(ctx, cronJob)
				return r.createJobForGroup(ctx, osint, jobName, resolvedFiles, assets, createdJobs, existingJobs)
			}
			*existingJobs++
		} else {
			return err
		}
	} else {
		job := &batchv1.Job{}
		err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: targetNamespace}, job)
		if err != nil && errors.IsNotFound(err) {
			newJob, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, jobName, targetNamespace, "osint", r.ControllerNamespace)
			if err != nil {
				return err
			}
			if err := r.Create(ctx, newJob); err != nil {
				return err
			}
			*createdJobs++
		} else if err == nil {
			*existingJobs++
		} else {
			return err
		}
	}
	return nil
}

func (r *OsintReconciler) reconcileSingleJob(ctx context.Context, osint *securityv1alpha1.Osint, items []securityv1alpha1.AssetItem, resolvedFiles []string) (ctrl.Result, error) {
	c, e := 0, 0
	err := r.createJobForGroup(ctx, osint, osint.Name, resolvedFiles, items, &c, &e)
	if err != nil {
		return ctrl.Result{}, err
	}
	osint.Status.ResolvedAssets = items
	return ctrl.Result{RequeueAfter: 30 * time.Second}, r.Status().Update(ctx, osint)
}

func (r *OsintReconciler) reconcileSingleJobWithTarget(ctx context.Context, osint *securityv1alpha1.Osint, resolvedFiles []string) (ctrl.Result, error) {
	c, e := 0, 0
	_ = r.createJobForGroup(ctx, osint, osint.Name, resolvedFiles, []securityv1alpha1.AssetItem{}, &c, &e)
	return ctrl.Result{RequeueAfter: 30 * time.Second}, r.Status().Update(ctx, osint)
}

func generateJobName(osintName, groupName string, groupIndex int) string {
	sanitizedGroupName := sanitizeName(groupName)
	if sanitizedGroupName != "" {
		return fmt.Sprintf("%s-%s", osintName, sanitizedGroupName)
	}
	return fmt.Sprintf("%s-group-%d", osintName, groupIndex)
}

func sanitizeName(name string) string {
	result := ""
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result += string(char)
		} else if char >= 'A' && char <= 'Z' {
			result += string(char + 32)
		} else if char == ' ' || char == '_' || char == '-' {
			result += "-"
		}
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

type OsintAdapter struct {
	osint          *securityv1alpha1.Osint
	resolvedFiles  []string
	resolvedAssets []securityv1alpha1.AssetItem
}

func (a *OsintAdapter) GetName() string              { return a.osint.Name }
func (a *OsintAdapter) GetNamespace() string         { return a.osint.Namespace }
func (a *OsintAdapter) GetKubeObject() client.Object { return a.osint }

func (a *OsintAdapter) GetSpec() *ResourceSpec {
	envVars := make([]corev1.EnvVar, len(a.osint.Spec.AdditionalEnv))
	for i, ev := range a.osint.Spec.AdditionalEnv {
		envVars[i] = corev1.EnvVar{Name: ev.Name, Value: ev.Value}
	}
	return &ResourceSpec{
		Tool:          a.osint.Spec.Tool,
		Target:        a.osint.Spec.Target,
		Category:      a.osint.Spec.Category,
		Args:          a.osint.Spec.Args,
		HTTPProxy:     a.osint.Spec.HTTPProxy,
		AdditionalEnv: envVars,
		Debug:         a.osint.Spec.Debug,
		Periodic:      a.osint.Spec.Periodic,
		Schedule:      a.osint.Spec.Schedule,
		Port:          a.osint.Spec.Port,
		Files:         a.resolvedFiles,
		Assets:        a.resolvedAssets,
	}
}

func (a *OsintAdapter) GetStatus() *ResourceStatus {
	return &ResourceStatus{
		State:        a.osint.Status.State,
		LastExecuted: a.osint.Status.LastExecuted,
	}
}

func (r *OsintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Osint{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
