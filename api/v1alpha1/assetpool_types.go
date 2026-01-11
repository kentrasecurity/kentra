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

// AssetItem defines a single asset in the pool
type AssetItem struct {
	// Type es: "username", "email", "phone"
	Type string `json:"type"`

	// Value deve essere una stringa singola
	Value string `json:"value"`
}

// AssetGroup represents a logical grouping of multiple asset sets
// For example: a person with their primary and secondary contact info
type AssetGroup struct {
	Name   string      `json:"name,omitempty"`
	Assets []AssetItem `json:"assets,omitempty"`
}

// AssetPoolSpec defines the desired state of AssetPool
type AssetPoolSpec struct {
	Description string       `json:"description,omitempty"`
	Groups      []AssetGroup `json:"group,omitempty"`
	// Manteniamo Items per compatibilità se lo usi ancora all'esterno dei gruppi
	Items []AssetItem `json:"items,omitempty"`
}

// AssetPoolStatus defines the observed state of AssetPool
type AssetPoolStatus struct {
	ItemCount          int    `json:"itemCount,omitempty"`
	GroupCount         int    `json:"groupCount,omitempty"`
	LastUpdated        string `json:"lastUpdated,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=ap,singular=assetpool
//+kubebuilder:printcolumn:name="Items",type=integer,JSONPath=`.status.itemCount`
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AssetPool is the Schema for the assetpools API
type AssetPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AssetPoolSpec   `json:"spec,omitempty"`
	Status AssetPoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AssetPoolList contains a list of AssetPool
type AssetPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AssetPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AssetPool{}, &AssetPoolList{})
}
