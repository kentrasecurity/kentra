package controller

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
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// OsintReconciler reconciles a Osint object
type OsintReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Configurator *ToolsConfigurator
}

// +kubebuilder:rbac:groups=kentra.sh,resources=osints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kentra.sh,resources=osints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kentra.sh,resources=osints/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile implements reconciliation for Osint resources
func (r *OsintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Load tool configurations from ConfigMap if not already loaded
	if err := r.Configurator.LoadConfig(ctx); err != nil {
		log.Error(err, "Failed to load tool configurations, retrying...")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// Fetch the Osint resource
	osint := &securityv1alpha1.Osint{}
	if err := r.Get(ctx, req.NamespacedName, osint); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Osint resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Osint")
		return ctrl.Result{}, err
	}

	// Ensure labels are set
	if osint.Labels == nil {
		osint.Labels = make(map[string]string)
	}
	needsUpdate := false
	if osint.Labels["kentra.sh/resource-type"] != "attack" {
		osint.Labels["kentra.sh/resource-type"] = "attack"
		needsUpdate = true
	}

	// Update the resource if labels were modified
	if needsUpdate {
		if err := r.Update(ctx, osint); err != nil {
			log.Error(err, "Failed to update Osint labels")
			return ctrl.Result{}, err
		}
	}

	// Initialize slices
	var resolvedFiles []string
	var resolvedAssets []securityv1alpha1.AssetItem

	// Resolve StoragePool reference if provided
	if osint.Spec.StoragePool != "" {
		sg := &securityv1alpha1.StoragePool{}
		sgNN := types.NamespacedName{Name: osint.Spec.StoragePool, Namespace: osint.Namespace}
		if err := r.Get(ctx, sgNN, sg); err != nil {
			log.Error(err, "Failed to get referenced StoragePool", "StoragePool", osint.Spec.StoragePool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		// Set files from StoragePool
		resolvedFiles = sg.Spec.Files
		log.Info("Resolved StoragePool", "StoragePool", osint.Spec.StoragePool, "filesCount", len(resolvedFiles))
	}

	// Resolve AssetPool reference if provided
	if osint.Spec.AssetPool != "" {
		ap := &securityv1alpha1.AssetPool{}
		apNN := types.NamespacedName{Name: osint.Spec.AssetPool, Namespace: osint.Namespace}
		if err := r.Get(ctx, apNN, ap); err != nil {
			log.Error(err, "Failed to get referenced AssetPool", "AssetPool", osint.Spec.AssetPool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		if len(ap.Spec.Items) == 0 {
			err := fmt.Errorf("assetPool %s has no items", osint.Spec.AssetPool)
			log.Error(err, "Invalid AssetPool")
			return ctrl.Result{}, err
		}

		// Collect all assets for processing
		resolvedAssets = make([]securityv1alpha1.AssetItem, len(ap.Spec.Items))
		for i, item := range ap.Spec.Items {
			resolvedAssets[i] = securityv1alpha1.AssetItem{
				Type:  item.Type,
				Value: item.Value,
			}
		}

		// Update status with all resolved assets
		osint.Status.ResolvedAssets = resolvedAssets

		// For backward compatibility, set first item as primary
		firstItem := ap.Spec.Items[0]
		osint.Spec.Target = firstItem.Value
		osint.Status.ResolvedAsset = firstItem.Value
		osint.Status.ResolvedAssetType = firstItem.Type

		log.Info("Resolved AssetPool",
			"AssetPool", osint.Spec.AssetPool,
			"totalAssets", len(resolvedAssets),
			"primaryAsset", firstItem.Value,
			"primaryType", firstItem.Type)
	}

	// Resolve TargetPool reference if provided
	if osint.Spec.TargetPool != "" {
		tg := &securityv1alpha1.TargetPool{}
		tgNN := types.NamespacedName{Name: osint.Spec.TargetPool, Namespace: osint.Namespace}
		if err := r.Get(ctx, tgNN, tg); err != nil {
			log.Error(err, "Failed to get referenced TargetPool", "TargetPool", osint.Spec.TargetPool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		// Set target and port from TargetPool
		osint.Spec.Target = tg.Spec.Target
		if tg.Spec.Port != "" && osint.Spec.Port == "" {
			osint.Spec.Port = tg.Spec.Port
		}
		// Update resolved status
		osint.Status.ResolvedTarget = tg.Spec.Target
		osint.Status.ResolvedPort = tg.Spec.Port
		log.Info("Resolved TargetPool", "TargetPool", osint.Spec.TargetPool, "target", tg.Spec.Target)
	} else {
		// Use direct target and port
		osint.Status.ResolvedTarget = osint.Spec.Target
		osint.Status.ResolvedPort = osint.Spec.Port
	}

	// Validate that either Target, TargetPool, or AssetPool is set
	if osint.Spec.Target == "" {
		err := fmt.Errorf("neither target, targetPool, nor assetPool specified")
		log.Error(err, "Invalid Osint resource")
		return ctrl.Result{}, err
	}

	// Determine target namespace
	targetNamespace := osint.Namespace

	// Generate names for Job/CronJob
	jobName := osint.Name
	cronJobName := fmt.Sprintf("%s-cronjob", osint.Name)

	// Convert to SecurityResource adapter
	adapter := &OsintAdapter{
		osint:          osint,
		resolvedFiles:  resolvedFiles,
		resolvedAssets: resolvedAssets,
	}

	log.Info("Building resource with spec",
		"tool", adapter.GetSpec().Tool,
		"target", adapter.GetSpec().Target,
		"filesCount", len(adapter.GetSpec().Files),
		"assetsCount", len(adapter.GetSpec().Assets))

	// Check if we need to create a job or cronjob
	if osint.Spec.Periodic {
		// Create or update CronJob
		cronJob := &batchv1.CronJob{}
		cronJobNN := types.NamespacedName{Name: cronJobName, Namespace: targetNamespace}

		err := r.Get(ctx, cronJobNN, cronJob)
		if err != nil && errors.IsNotFound(err) {
			// Create new CronJob
			cronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, cronJobName, targetNamespace, "osint")
			if err != nil {
				log.Error(err, "Failed to build CronJob")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "Failed to create CronJob")
				return ctrl.Result{}, err
			}
			log.Info("Created new CronJob", "CronJob", cronJobName)
			osint.Status.State = "Running"
			osint.Status.JobName = cronJobName
			osint.Status.LastExecuted = time.Now().Format(time.RFC3339)
		} else if err != nil {
			log.Error(err, "Failed to get CronJob")
			return ctrl.Result{}, err
		} else {
			// CronJob exists - check if it needs to be recreated due to spec change
			currentGeneration := fmt.Sprintf("%d", osint.Generation)
			existingGeneration := cronJob.Annotations["kentra.sh/parent-generation"]

			if existingGeneration != currentGeneration {
				log.Info("Osint spec changed, deleting and recreating CronJob",
					"CronJob", cronJobName,
					"oldGeneration", existingGeneration,
					"newGeneration", currentGeneration)

				// Delete the existing CronJob
				if err := r.Delete(ctx, cronJob); err != nil {
					log.Error(err, "Failed to delete outdated CronJob")
					return ctrl.Result{}, err
				}

				// Create new CronJob with updated spec
				newCronJob, err := BuildCronJob(ctx, adapter, r.Scheme, r.Configurator, cronJobName, targetNamespace, "osint")
				if err != nil {
					log.Error(err, "Failed to build new CronJob")
					return ctrl.Result{}, err
				}
				if err := r.Create(ctx, newCronJob); err != nil {
					log.Error(err, "Failed to create new CronJob")
					return ctrl.Result{}, err
				}
				log.Info("Recreated CronJob with updated spec", "CronJob", cronJobName)
				osint.Status.State = "Running"
				osint.Status.JobName = cronJobName
				osint.Status.LastExecuted = time.Now().Format(time.RFC3339)
			} else {
				log.Info("CronJob already exists and is up to date", "CronJob", cronJobName)
			}
		}
	} else {
		// Create one-time Job
		job := &batchv1.Job{}
		jobNN := types.NamespacedName{Name: jobName, Namespace: targetNamespace}

		err := r.Get(ctx, jobNN, job)
		if err != nil && errors.IsNotFound(err) {
			// Create new Job
			job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, jobName, targetNamespace, "osint")
			if err != nil {
				log.Error(err, "Failed to build Job")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Failed to create Job")
				return ctrl.Result{}, err
			}
			log.Info("Created new Job", "Job", jobName)
			osint.Status.State = "Running"
			osint.Status.JobName = jobName
			osint.Status.LastExecuted = time.Now().Format(time.RFC3339)
		} else if err != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		} else {
			// Job exists - check if it needs to be recreated due to spec change
			currentGeneration := fmt.Sprintf("%d", osint.Generation)
			existingGeneration := job.Annotations["kentra.sh/parent-generation"]

			if existingGeneration != currentGeneration {
				log.Info("Osint spec changed, deleting and recreating Job",
					"Job", jobName,
					"oldGeneration", existingGeneration,
					"newGeneration", currentGeneration)

				// Delete the existing Job
				propagationPolicy := metav1.DeletePropagationBackground
				if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil {
					log.Error(err, "Failed to delete outdated Job")
					return ctrl.Result{}, err
				}

				log.Info("Deleted outdated Job, will recreate on next reconcile", "Job", jobName)
				return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
			} else {
				log.Info("Job already exists and is up to date", "Job", jobName)
			}
		}
	}

	// Update status with resolved target and port
	if err := r.Status().Update(ctx, osint); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// OsintAdapter adapts Osint to SecurityResource interface
type OsintAdapter struct {
	osint          *securityv1alpha1.Osint
	resolvedFiles  []string
	resolvedAssets []securityv1alpha1.AssetItem
}

func (a *OsintAdapter) GetName() string {
	return a.osint.Name
}

func (a *OsintAdapter) GetNamespace() string {
	return a.osint.Namespace
}

func (a *OsintAdapter) GetSpec() *ResourceSpec {
	envVars := make([]corev1.EnvVar, len(a.osint.Spec.AdditionalEnv))
	for i, ev := range a.osint.Spec.AdditionalEnv {
		envVars[i] = corev1.EnvVar{
			Name:  ev.Name,
			Value: ev.Value,
		}
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

func (a *OsintAdapter) GetKubeObject() client.Object {
	return a.osint
}

// SetupWithManager sets up the controller with the Manager.
func (r *OsintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.Osint{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
