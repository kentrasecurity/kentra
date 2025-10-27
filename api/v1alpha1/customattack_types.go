package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomAttackSpec defines the desired state of CustomAttack
type CustomAttackSpec struct {
	// Description is a human-readable description of what this attack does
	// +optional
	Description string `json:"description,omitempty"`

	// Tool is the full container image reference (e.g., busybox:latest, myregistry/tool:v1.0)
	// +kubebuilder:validation:Required
	// +kubebuilder:example="busybox:latest"
	Tool string `json:"tool"`

	// Command overrides the entrypoint of the container
	// +optional
	Command []string `json:"command,omitempty"`

	// Args contains arguments to pass to the tool
	// +optional
	Args []string `json:"args,omitempty"`

	// Periodic indicates if this should run on a schedule
	// +optional
	Periodic bool `json:"periodic,omitempty"`

	// Schedule is the cron schedule (only used if Periodic is true)
	// +optional
	// +kubebuilder:default="0 */6 * * *"
	Schedule string `json:"schedule,omitempty"`

	// ImagePullPolicy for the tool container
	// +optional
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +kubebuilder:default="IfNotPresent"
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets for pulling private images
	// +optional
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// Resources defines resource requests and limits
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// EnvVars are environment variables for the tool container
	// +optional
	EnvVars []EnvVar `json:"envVars,omitempty"`

	// SecurityContext for the pod
	// +optional
	SecurityContext *SecurityContext `json:"securityContext,omitempty"`
}

// EnvVar represents an environment variable
type EnvVar struct {
	// Name of the environment variable
	Name string `json:"name"`
	// Value of the environment variable
	Value string `json:"value"`
}

// SecurityContext defines security settings for the pod
type SecurityContext struct {
	// RunAsUser is the UID to run the entrypoint
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`

	// RunAsGroup is the GID to run the entrypoint
	// +optional
	RunAsGroup *int64 `json:"runAsGroup,omitempty"`

	// FSGroup is the group ID for volume ownership
	// +optional
	FSGroup *int64 `json:"fsGroup,omitempty"`

	// RunAsNonRoot indicates that the container must run as a non-root user
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`

	// Capabilities to add or drop
	// +optional
	Capabilities *Capabilities `json:"capabilities,omitempty"`
}

// Capabilities defines Linux capabilities
type Capabilities struct {
	// Add capabilities
	// +optional
	Add []string `json:"add,omitempty"`

	// Drop capabilities
	// +optional
	Drop []string `json:"drop,omitempty"`
}

// CustomAttackStatus defines the observed state of CustomAttack
type CustomAttackStatus struct {
	// LastExecuted is the timestamp of last execution
	// +optional
	LastExecuted string `json:"lastExecuted,omitempty"`

	// JobName is the name of the created job or cronjob
	// +optional
	JobName string `json:"jobName,omitempty"`

	// State represents the current state
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	State string `json:"state,omitempty"`

	// Message provides additional information about the state
	// +optional
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=ca
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
//+kubebuilder:printcolumn:name="Tool",type=string,JSONPath=`.spec.tool`
//+kubebuilder:printcolumn:name="Periodic",type=boolean,JSONPath=`.spec.periodic`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CustomAttack is the Schema for the customattacks API
type CustomAttack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomAttackSpec   `json:"spec,omitempty"`
	Status CustomAttackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CustomAttackList contains a list of CustomAttack
type CustomAttackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomAttack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CustomAttack{}, &CustomAttackList{})
}
