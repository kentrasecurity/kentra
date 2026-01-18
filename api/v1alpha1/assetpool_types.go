package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AssetPoolSpec defines the desired state of AssetPool
type AssetPoolSpec struct {
	Description string `json:"description,omitempty"`

	// Asset is a map where keys are asset types (username, email, etc.)
	// and values are arrays of strings
	// +kubebuilder:validation:Required
	Asset map[string][]string `json:"asset"`
}

// AssetPoolStatus defines the observed state of AssetPool
type AssetPoolStatus struct {
	TotalAssets        int    `json:"totalAssets,omitempty"`
	LastUpdated        string `json:"lastUpdated,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
