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
	// Type specifies the type of asset (e.g., username, email, domain, ip)
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Value is the actual value of the asset
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

// AssetPoolSpec defines the desired state of AssetPool
type AssetPoolSpec struct {
	// Description is an optional description of the asset pool
	// +optional
	Description string `json:"description,omitempty"`

	// Items is the list of assets in this pool
	// +kubebuilder:validation:Required
	Items []AssetItem `json:"items"`
}

// AssetPoolStatus defines the observed state of AssetPool
type AssetPoolStatus struct {
	// LastUpdated is the timestamp of last update
	// +optional
	LastUpdated string `json:"lastUpdated,omitempty"`

	// ItemCount is the number of items in this asset pool
	// +optional
	ItemCount int `json:"itemCount,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
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
