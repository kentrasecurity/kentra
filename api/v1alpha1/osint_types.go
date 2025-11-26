package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OsintSpec defines the desired state of Osint
type OsintSpec struct {
	// Target specifies the target username(s) for OSINT operations
	Target string `json:"target,omitempty"`

	// TargetGroup references a TargetGroup resource
	TargetGroup string `json:"targetGroup,omitempty"`

	// Tool specifies which OSINT tool to use (e.g., sherlock)
	Tool string `json:"tool"`

	// FileName specifies the output file name
	FileName string `json:"fileName,omitempty"`

	// Periodic indicates if this should run as a CronJob
	Periodic bool `json:"periodic,omitempty"`

	// Schedule defines the cron schedule (required if Periodic is true)
	Schedule string `json:"schedule,omitempty"`

	// HTTPProxy specifies an HTTP proxy to use
	HTTPProxy string `json:"httpProxy,omitempty"`

	// Capabilities specifies additional Linux capabilities
	Capabilities []string `json:"capabilities,omitempty"`

	// AdditionalEnv specifies additional environment variables
	AdditionalEnv []EnvVar `json:"additionalEnv,omitempty"`

	// Args specifies additional command-line arguments
	Args []string `json:"args,omitempty"`

	// Debug enables debug mode
	Debug bool `json:"debug,omitempty"`

	// Category classifies the type of OSINT operation
	Category string `json:"category,omitempty"`

	// Port specifies a port (if applicable)
	Port string `json:"port,omitempty"`

	// StorageGroup references a StorageGroup resource for file inputs
	StorageGroup string `json:"storageGroup,omitempty"`
}

// OsintStatus defines the observed state of Osint
type OsintStatus struct {
	// LastExecuted timestamp of the last execution
	LastExecuted string `json:"lastExecuted,omitempty"`

	// JobName is the name of the created Job or CronJob
	JobName string `json:"jobName,omitempty"`

	// State represents the current state of the attack
	State string `json:"state,omitempty"`

	// ResultsLocation indicates where results are stored
	ResultsLocation string `json:"resultsLocation,omitempty"`

	// ResolvedTarget is the actual target after TargetGroup resolution
	ResolvedTarget string `json:"resolvedTarget,omitempty"`

	// ResolvedPort is the actual port after TargetGroup resolution
	ResolvedPort string `json:"resolvedPort,omitempty"`
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
