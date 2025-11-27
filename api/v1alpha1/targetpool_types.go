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

// TargetPoolSpec defines the desired state of TargetPool
type TargetPoolSpec struct {
	// Description is an optional description of the target group
	// +optional
	Description string `json:"description,omitempty"`

	// Target is the IP address, CIDR, or hostname to target
	// +kubebuilder:validation:Required
	Target string `json:"target"`

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
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=tp,singular=targetpool
//+kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target`
//+kubebuilder:printcolumn:name="Port",type=string,JSONPath=`.spec.port`
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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
