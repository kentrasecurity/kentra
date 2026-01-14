package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AssetItem struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type AssetPoolItem struct {
	Name   string      `json:"name"`
	Assets []AssetItem `json:"assets"`
}

type AssetPoolSpec struct {
	Description string `json:"description,omitempty"`
	// This matches the 'pool:' key in your YAML
	Pool []AssetPoolItem `json:"pool,omitempty"`
}

type AssetPoolStatus struct {
	// Total number of groups (AssetPoolItems)
	GroupCount int `json:"groupCount,omitempty"`
	// Total number of individual assets across all groups
	TotalAssets        int    `json:"totalAssets,omitempty"`
	LastUpdated        string `json:"lastUpdated,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=ap,singular=assetpool
//+kubebuilder:printcolumn:name="Groups",type=integer,JSONPath=`.status.groupCount`
//+kubebuilder:printcolumn:name="Assets",type=integer,JSONPath=`.status.totalAssets`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type AssetPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AssetPoolSpec   `json:"spec,omitempty"`
	Status AssetPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type AssetPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AssetPool `json:"items"`
}

func init() {
	// This line registers the types so the Manager knows they exist
	SchemeBuilder.Register(&AssetPool{}, &AssetPoolList{})
}
