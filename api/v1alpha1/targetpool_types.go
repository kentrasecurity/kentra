package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TargetSpec defines a named target with endpoints and ports
type TargetSpec struct {
	// Name is the identifier for this target
	Name string `json:"name"`

	// Endpoint can be a single value or array of IPs, CIDRs, or domains
	// +kubebuilder:validation:MinItems=1
	Endpoint []string `json:"endpoint"`

	// Port can be a single value or array of ports or port ranges (e.g., "22", "80", "22-443")
	// +kubebuilder:validation:MinItems=1
	Port []string `json:"port"`
}

// TargetPoolSpec defines the desired state of TargetPool
type TargetPoolSpec struct {
	Description string       `json:"description,omitempty"`
	Targets     []TargetSpec `json:"targets"`
}

// TargetPoolStatus defines the observed state of TargetPool
type TargetPoolStatus struct {
	TotalTargets       int    `json:"totalTargets,omitempty"`
	LastUpdated        string `json:"lastUpdated,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TargetPool is the Schema for the targetpools API
type TargetPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TargetPoolSpec   `json:"spec,omitempty"`
	Status TargetPoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TargetPoolList contains a list of TargetPool
type TargetPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TargetPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TargetPool{}, &TargetPoolList{})
}
