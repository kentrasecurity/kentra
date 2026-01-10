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

// OsintReconciler reconciles a Osint object
type OsintReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Configurator        *ToolsConfigurator
	ControllerNamespace string
}

// +kubebuilder:rbac:groups=kentra.sh,resources=osints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kentra.sh,resources=osints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kentra.sh,resources=osints/finalizers,verbs=update
// +kubebuilder:rbac:groups=kttack.io,resources=targetpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=kttack.io,resources=assetpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=kttack.io,resources=storagepools,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list

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

	// Check if namespace is managed by Kentra
	isManaged, err := isNamespaceManagedByKentra(ctx, r.Client, osint.Namespace)
	if err != nil {
		log.Error(err, "Failed to check if namespace is managed by Kentra", "namespace", osint.Namespace)
		return ctrl.Result{}, err
	}
	if !isManaged {
		log.Error(fmt.Errorf("namespace not managed by Kentra"), "Cannot create Osint in namespace without 'managed-by-kentra' annotation", "namespace", osint.Namespace)
		return ctrl.Result{}, fmt.Errorf("namespace %s is not managed by Kentra (missing 'managed-by-kentra' annotation)", osint.Namespace)
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

	// Resolve StoragePool reference if provided
	if osint.Spec.StoragePool != "" {
		sg := &securityv1alpha1.StoragePool{}
		sgNN := types.NamespacedName{Name: osint.Spec.StoragePool, Namespace: osint.Namespace}
		if err := r.Get(ctx, sgNN, sg); err != nil {
			log.Error(err, "Failed to get referenced StoragePool", "StoragePool", osint.Spec.StoragePool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		resolvedFiles = sg.Spec.Files
		log.Info("Resolved StoragePool", "StoragePool", osint.Spec.StoragePool, "filesCount", len(resolvedFiles))
	}

	// Resolve AssetPool and create jobs
	if osint.Spec.AssetPool != "" {
		ap := &securityv1alpha1.AssetPool{}
		apNN := types.NamespacedName{Name: osint.Spec.AssetPool, Namespace: osint.Namespace}
		if err := r.Get(ctx, apNN, ap); err != nil {
			log.Error(err, "Failed to get referenced AssetPool", "AssetPool", osint.Spec.AssetPool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		if len(ap.Spec.Groups) > 0 {
			// Multi-job mode: Create one job per group
			return r.reconcileMultipleJobs(ctx, osint, ap, resolvedFiles)
		} else if len(ap.Spec.Items) > 0 {
			// Single job mode: Use flat list
			return r.reconcileSingleJob(ctx, osint, ap.Spec.Items, resolvedFiles)
		} else {
			err := fmt.Errorf("assetPool %s has no items or groups", osint.Spec.AssetPool)
			log.Error(err, "Invalid AssetPool")
			return ctrl.Result{}, err
		}
	}

	// Handle TargetPool reference
	if osint.Spec.TargetPool != "" {
		tg := &securityv1alpha1.TargetPool{}
		tgNN := types.NamespacedName{Name: osint.Spec.TargetPool, Namespace: osint.Namespace}
		if err := r.Get(ctx, tgNN, tg); err != nil {
			log.Error(err, "Failed to get referenced TargetPool", "TargetPool", osint.Spec.TargetPool)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		osint.Spec.Target = tg.Spec.Target
		if tg.Spec.Port != "" && osint.Spec.Port == "" {
			osint.Spec.Port = tg.Spec.Port
		}
		osint.Status.ResolvedTarget = tg.Spec.Target
		osint.Status.ResolvedPort = tg.Spec.Port
	} else {
		osint.Status.ResolvedTarget = osint.Spec.Target
		osint.Status.ResolvedPort = osint.Spec.Port
	}

	// Validate that either Target, TargetPool, or AssetPool is set
	if osint.Spec.Target == "" {
		err := fmt.Errorf("neither target, targetPool, nor assetPool specified")
		log.Error(err, "Invalid Osint resource")
		return ctrl.Result{}, err
	}

	// Standard single job creation (when no AssetPool)
	return r.reconcileSingleJobWithTarget(ctx, osint, resolvedFiles)
}

// reconcileMultipleJobs creates one job per asset set in each group
// reconcileMultipleJobs creates one job per asset set in each group
func (r *OsintReconciler) reconcileMultipleJobs(ctx context.Context, osint *securityv1alpha1.Osint, ap *securityv1alpha1.AssetPool, resolvedFiles []string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	createdJobs := 0
	existingJobs := 0
	var jobNames []string
	totalAssetSets := 0

	log.Info("Creating multiple jobs from AssetPool groups",
		"AssetPool", osint.Spec.AssetPool,
		"groupCount", len(ap.Spec.Groups))

	// Process each group
	for groupIndex, group := range ap.Spec.Groups {
		if len(group.AssetSets) > 0 {
			for _, assetSet := range group.AssetSets {
				if len(assetSet.Assets) == 0 {
					continue
				}
				totalAssetSets++
				jobName := generateJobName(osint.Name, group.Name, groupIndex, assetSet.Name)
				jobNames = append(jobNames, jobName)

				// FIXED: Removed targetNamespace, groupIndex, group.Name, and assetSet.Name from arguments
				if err := r.createJobForGroup(ctx, osint, jobName, resolvedFiles, assetSet.Assets, &createdJobs, &existingJobs); err != nil {
					log.Error(err, "Failed to create job for asset set")
					continue
				}
			}
		} else if len(group.Assets) > 0 {
			totalAssetSets++
			jobName := generateJobName(osint.Name, group.Name, groupIndex, "default")
			jobNames = append(jobNames, jobName)

			// FIXED: Removed targetNamespace, groupIndex, group.Name, and "default" from arguments
			if err := r.createJobForGroup(ctx, osint, jobName, resolvedFiles, group.Assets, &createdJobs, &existingJobs); err != nil {
				log.Error(err, "Failed to create job for legacy asset group")
				continue
			}
		}
	}

	// Update status logic...
	osint.Status.State = "Running"
	osint.Status.JobName = fmt.Sprintf("%d jobs created", createdJobs+existingJobs)
	osint.Status.LastExecuted = time.Now().Format(time.RFC3339)

	// Store all resolved assets for status tracking
	var allAssets []securityv1alpha1.AssetItem
	for _, group := range ap.Spec.Groups {
		if len(group.AssetSets) > 0 {
			for _, assetSet := range group.AssetSets {
				allAssets = append(allAssets, assetSet.Assets...)
			}
		} else {
			allAssets = append(allAssets, group.Assets...)
		}
	}
	osint.Status.ResolvedAssets = allAssets

	if len(allAssets) > 0 {
		osint.Spec.Target = allAssets[0].Value
		osint.Status.ResolvedAsset = allAssets[0].Value
		osint.Status.ResolvedAssetType = allAssets[0].Type
	}

	if err := r.Status().Update(ctx, osint); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// createJobForGroup: parametri puliti
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

// generateJobName creates a unique job name for an asset set
func generateJobName(osintName, groupName string, groupIndex int, assetSetName string) string {
	sanitizedGroupName := sanitizeName(groupName)
	sanitizedAssetName := sanitizeName(assetSetName)

	if groupName != "" && sanitizedGroupName != "" {
		return fmt.Sprintf("%s-%s-%s", osintName, sanitizedGroupName, sanitizedAssetName)
	}
	return fmt.Sprintf("%s-g%d-%s", osintName, groupIndex, sanitizedAssetName)
}

// reconcileSingleJob creates a single job from a flat list of assets
func (r *OsintReconciler) reconcileSingleJob(ctx context.Context, osint *securityv1alpha1.Osint, items []securityv1alpha1.AssetItem, resolvedFiles []string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	resolvedAssets := make([]securityv1alpha1.AssetItem, len(items))
	copy(resolvedAssets, items)

	targetNamespace := osint.Namespace
	jobName := osint.Name

	adapter := &OsintAdapter{
		osint:          osint,
		resolvedFiles:  resolvedFiles,
		resolvedAssets: resolvedAssets,
	}

	log.Info("Creating single job from flat asset list",
		"jobName", jobName,
		"assetsCount", len(resolvedAssets))

	job := &batchv1.Job{}
	jobNN := types.NamespacedName{Name: jobName, Namespace: targetNamespace}
	err := r.Get(ctx, jobNN, job)

	if err != nil && errors.IsNotFound(err) {
		job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, jobName, targetNamespace, "osint", r.ControllerNamespace)
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
		log.Info("Job already exists", "Job", jobName)
	}

	// Update status
	osint.Status.ResolvedAssets = resolvedAssets
	if len(resolvedAssets) > 0 {
		osint.Spec.Target = resolvedAssets[0].Value
		osint.Status.ResolvedAsset = resolvedAssets[0].Value
		osint.Status.ResolvedAssetType = resolvedAssets[0].Type
	}

	if err := r.Status().Update(ctx, osint); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileSingleJobWithTarget creates a single job with a direct target
func (r *OsintReconciler) reconcileSingleJobWithTarget(ctx context.Context, osint *securityv1alpha1.Osint, resolvedFiles []string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	targetNamespace := osint.Namespace
	jobName := osint.Name

	adapter := &OsintAdapter{
		osint:          osint,
		resolvedFiles:  resolvedFiles,
		resolvedAssets: []securityv1alpha1.AssetItem{},
	}

	log.Info("Creating single job with direct target", "jobName", jobName, "target", osint.Spec.Target)

	job := &batchv1.Job{}
	jobNN := types.NamespacedName{Name: jobName, Namespace: targetNamespace}
	err := r.Get(ctx, jobNN, job)

	if err != nil && errors.IsNotFound(err) {
		job, err := BuildJob(ctx, adapter, r.Scheme, r.Configurator, r.Client, jobName, targetNamespace, "osint", r.ControllerNamespace)
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
	}

	if err := r.Status().Update(ctx, osint); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// sanitizeName sanitizes a string to be used as a Kubernetes resource name
func sanitizeName(name string) string {
	// Kubernetes names must be lowercase alphanumeric or '-'
	// Maximum 253 characters
	result := ""
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result += string(char)
		} else if char >= 'A' && char <= 'Z' {
			result += string(char + 32) // Convert to lowercase
		} else if char == ' ' || char == '_' {
			result += "-"
		}
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// getAssetTypeCounts returns a count of assets by type for logging
func getAssetTypeCounts(assets []securityv1alpha1.AssetItem) map[string]int {
	counts := make(map[string]int)
	for _, asset := range assets {
		counts[asset.Type]++
	}
	return counts
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
