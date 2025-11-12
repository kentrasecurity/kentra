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

// TargetGroupSpec defines the desired state of TargetGroup
type TargetGroupSpec struct {
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

// TargetGroupStatus defines the observed state of TargetGroup
type TargetGroupStatus struct {
	// LastUpdated is the timestamp of last update
	// +optional
	LastUpdated string `json:"lastUpdated,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=tg,singular=targetgroup
//+kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target`
//+kubebuilder:printcolumn:name="Port",type=string,JSONPath=`.spec.port`
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TargetGroup is the Schema for the targetgroups API
type TargetGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TargetGroupSpec   `json:"spec,omitempty"`
	Status TargetGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TargetGroupList contains a list of TargetGroup
type TargetGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TargetGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TargetGroup{}, &TargetGroupList{})
}
