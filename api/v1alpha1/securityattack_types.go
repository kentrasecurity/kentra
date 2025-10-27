package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityAttackSpec defines the desired state of SecurityAttack
type SecurityAttackSpec struct {
	// AttackType specifies the type of security test
	// +kubebuilder:validation:Enum=Enumeration;Vulnerability;Exploitation
	AttackType string `json:"attackType"`

	// Target is the IP, CIDR, or hostname to attack
	// +kubebuilder:validation:Required
	Target string `json:"target"`

	// Tool is the security tool to use
	// +kubebuilder:validation:Required
	Tool string `json:"tool"`

	// Periodic indicates if this should run on a schedule
	// +optional
	Periodic bool `json:"periodic,omitempty"`

	// Schedule is the cron schedule (only used if Periodic is true)
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Args contains additional arguments for the tool
	// +optional
	Args []string `json:"args,omitempty"`

	// TargetNamespace is where jobs/cronjobs will be created
	// +optional
	// +kubebuilder:default="kttack-system"
	TargetNamespace string `json:"targetNamespace,omitempty"`
}

// SecurityAttackStatus defines the observed state of SecurityAttack
type SecurityAttackStatus struct {
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
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=sa
//+kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.attackType`
//+kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target`
//+kubebuilder:printcolumn:name="Tool",type=string,JSONPath=`.spec.tool`
//+kubebuilder:printcolumn:name="Periodic",type=boolean,JSONPath=`.spec.periodic`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SecurityAttack is the Schema for the securityattacks API
type SecurityAttack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityAttackSpec   `json:"spec,omitempty"`
	Status SecurityAttackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecurityAttackList contains a list of SecurityAttack
type SecurityAttackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityAttack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecurityAttack{}, &SecurityAttackList{})
}
