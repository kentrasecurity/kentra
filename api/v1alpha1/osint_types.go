package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OsintSpec defines the desired state of Osint
type OsintSpec struct {
	// TargetPool references a TargetPool resource
	// +optional
	TargetPool string `json:"targetPool,omitempty"`

	// AssetPool references an AssetPool resource for OSINT targets
	// +optional
	AssetPool string `json:"assetPool,omitempty"`

	// Tool specifies which OSINT tool to use (e.g., sherlock)
	// +kubebuilder:validation:Required
	Tool string `json:"tool"`

	// FileName specifies the output file name
	// +optional
	FileName string `json:"fileName,omitempty"`

	// Periodic indicates if this should run as a CronJob
	// +optional
	Periodic bool `json:"periodic,omitempty"`

	// Schedule defines the cron schedule (required if Periodic is true)
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// HTTPProxy specifies an HTTP proxy to use
	// +optional
	HTTPProxy string `json:"httpProxy,omitempty"`

	// Capabilities specifies additional Linux capabilities
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`

	// AdditionalEnv specifies additional environment variables
	// +optional
	AdditionalEnv []EnvVar `json:"additionalEnv,omitempty"`

	// Args specifies additional command-line arguments
	// +optional
	Args []string `json:"args,omitempty"`

	// Debug enables debug mode
	// +optional
	Debug bool `json:"debug,omitempty"`

	// Category classifies the type of OSINT operation
	// +optional
	Category string `json:"category,omitempty"`

	// Port specifies a port (if applicable)
	// +optional
	Port string `json:"port,omitempty"`

	// StoragePool references a StoragePool resource for file inputs
	// +optional
	StoragePool string `json:"storagePool,omitempty"`
}

// OsintStatus defines the observed state of Osint
type OsintStatus struct {
	// LastExecuted timestamp of the last execution
	// +optional
	LastExecuted string `json:"lastExecuted,omitempty"`

	// JobName is the name of the created Job or CronJob
	// +optional
	JobName string `json:"jobName,omitempty"`

	// State represents the current state of the attack
	// +optional
	State string `json:"state,omitempty"`

	// ResultsLocation indicates where results are stored
	// +optional
	ResultsLocation string `json:"resultsLocation,omitempty"`

	// ResolvedTarget is the actual target(s) after TargetPool resolution
	// +optional
	ResolvedTarget string `json:"resolvedTarget,omitempty"`

	// ResolvedPort is the actual port after TargetPool resolution
	// +optional
	ResolvedPort string `json:"resolvedPort,omitempty"`

	// ResolvedAsset is the resolved asset value from AssetPool
	// +optional
	ResolvedAsset string `json:"resolvedAsset,omitempty"`

	// ResolvedAssetType is the type of the resolved asset
	// +optional
	ResolvedAssetType string `json:"resolvedAssetType,omitempty"`

	// ResolvedAssets contains all resolved assets from AssetPool
	// +optional
	ResolvedAssets []AssetItem `json:"resolvedAssets,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// Osint is the Schema for the osints API
type Osint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OsintSpec   `json:"spec,omitempty"`
	Status OsintStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OsintList contains a list of Osint
type OsintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Osint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Osint{}, &OsintList{})
}
