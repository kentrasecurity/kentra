/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TargetSpec defines different types of targets
type TargetSpec struct {
	// IP addresses or CIDRs
	// +optional
	IP []string `json:"ip,omitempty"`

	// Hostnames or domains
	// +optional
	Hostname []string `json:"hostname,omitempty"`

	// URLs
	// +optional
	URL []string `json:"url,omitempty"`
}

// TargetPoolSpec defines the desired state of TargetPool
type TargetPoolSpec struct {
	// Description is an optional description of the target pool
	// +optional
	Description string `json:"description,omitempty"`

	// Target contains categorized targets
	// +kubebuilder:validation:Required
	Target TargetSpec `json:"target"`

	// Port is the port or port range to target (e.g., '22', '80,443', '8000-8100')
	// +optional
	Port string `json:"port,omitempty"`
}

// TargetPoolStatus defines the observed state of TargetPool
type TargetPoolStatus struct {
	// LastUpdated is the timestamp of last update
	// +optional
	LastUpdated string `json:"lastUpdated,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ItemCount is the total number of targets
	// +optional
	ItemCount int `json:"itemCount,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=tp,singular=targetpool,categories=pool
//+kubebuilder:printcolumn:name="Targets",type=integer,JSONPath=`.status.itemCount`
//+kubebuilder:printcolumn:name="Port",type=string,JSONPath=`.spec.port`
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TargetPool is the Schema for the targetpools API
type TargetPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TargetPoolSpec   `json:"spec,omitempty"`
	Status            TargetPoolStatus `json:"status,omitempty"`
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

// GetAllTargets returns all targets as a flat list
func (t *TargetPool) GetAllTargets() []string {
	var targets []string
	targets = append(targets, t.Spec.Target.IP...)
	targets = append(targets, t.Spec.Target.Hostname...)
	targets = append(targets, t.Spec.Target.URL...)
	return targets
}
