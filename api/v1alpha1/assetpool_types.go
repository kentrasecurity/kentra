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

// AssetItem represents a single asset with its type
type AssetItem struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// AssetSpec defines different types of assets
type AssetSpec struct {
	// Usernames
	// +optional
	Username []string `json:"username,omitempty"`

	// Email addresses
	// +optional
	Email []string `json:"email,omitempty"`

	// Phone numbers
	// +optional
	Phone []string `json:"phone,omitempty"`

	// Social media handles
	// +optional
	SocialMedia []string `json:"socialMedia,omitempty"`

	// Company names
	// +optional
	Company []string `json:"company,omitempty"`

	// URIs (for git repos, APIs, etc.)
	// +optional
	URI []string `json:"uri,omitempty"`

	// Description or notes
	// +optional
	Description string `json:"description,omitempty"`
}

// AssetPoolSpec defines the desired state of AssetPool
type AssetPoolSpec struct {
	// Description is an optional description of the asset pool
	// +optional
	Description string `json:"description,omitempty"`

	// Asset contains categorized assets
	// +kubebuilder:validation:Required
	Asset AssetSpec `json:"asset"`
}

// AssetPoolStatus defines the observed state of AssetPool
type AssetPoolStatus struct {
	// LastUpdated is the timestamp of last update
	// +optional
	LastUpdated string `json:"lastUpdated,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ItemCount is the total number of assets
	// +optional
	ItemCount int `json:"itemCount,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=ap,singular=assetpool,categories=pool
//+kubebuilder:printcolumn:name="Assets",type=integer,JSONPath=`.status.itemCount`
//+kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AssetPool is the Schema for the assetpools API
type AssetPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AssetPoolSpec   `json:"spec,omitempty"`
	Status            AssetPoolStatus `json:"status,omitempty"`
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

// GetAllAssets returns all assets as a flat list with type information
func (a *AssetPool) GetAllAssets() []AssetItem {
	var assets []AssetItem

	for _, username := range a.Spec.Asset.Username {
		assets = append(assets, AssetItem{Type: "username", Value: username})
	}
	for _, email := range a.Spec.Asset.Email {
		assets = append(assets, AssetItem{Type: "email", Value: email})
	}
	for _, phone := range a.Spec.Asset.Phone {
		assets = append(assets, AssetItem{Type: "phone", Value: phone})
	}
	for _, social := range a.Spec.Asset.SocialMedia {
		assets = append(assets, AssetItem{Type: "socialMedia", Value: social})
	}
	for _, company := range a.Spec.Asset.Company {
		assets = append(assets, AssetItem{Type: "company", Value: company})
	}
	for _, uri := range a.Spec.Asset.URI {
		assets = append(assets, AssetItem{Type: "uri", Value: uri})
	}

	return assets
}
