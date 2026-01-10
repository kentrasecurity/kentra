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

// NamedAssetSet represents a named collection of assets
type NamedAssetSet struct {
	// Name is the identifier for this asset set (e.g., "primary", "work", "personal")
	Name string `json:"name"`

	// Assets are the individual assets in this set
	Assets []AssetItem `json:"assets"`
}

// AssetGroup represents a logical grouping of multiple asset sets
// For example: a person with their primary and secondary contact info
type AssetGroup struct {
	// Name is a friendly identifier for this group (e.g., "john-doe")
	Name string `json:"name,omitempty"`

	// AssetSets contains multiple named sets of assets
	// Each set will generate its own job
	AssetSets []NamedAssetSet `json:"assetSets,omitempty"`

	// Assets is the legacy field for backward compatibility
	// When AssetSets is empty, this will be used as a single asset set
	Assets []AssetItem `json:"assets,omitempty"`
}

// AssetPoolSpec defines the desired state of AssetPool
type AssetPoolSpec struct {
	// Description is an optional description of the asset pool
	// +optional
	Description string `json:"description,omitempty"`

	// Items is the flat list of assets (legacy/simple mode)
	Items []AssetItem `json:"items,omitempty"`

	// Groups is the structured list of asset groups
	// When Groups is specified, Items is ignored
	Groups []AssetGroup `json:"groups,omitempty"`
}

// AssetPoolStatus defines the observed state of AssetPool
type AssetPoolStatus struct {
	ItemCount          int    `json:"itemCount,omitempty"`
	GroupCount         int    `json:"groupCount,omitempty"`
	TotalAssetSets     int    `json:"totalAssetSets,omitempty"`
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
