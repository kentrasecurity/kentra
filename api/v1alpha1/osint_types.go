package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AssetItem represents a single asset with its type and value
type AssetItem struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// OsintSpec defines the desired state of Osint
type OsintSpec struct {
	Tool          string   `json:"tool"`
	AssetPool     string   `json:"assetPool,omitempty"`
	StoragePool   string   `json:"storagePool,omitempty"`
	Category      string   `json:"category,omitempty"`
	Args          []string `json:"args,omitempty"`
	HTTPProxy     string   `json:"httpProxy,omitempty"`
	AdditionalEnv []EnvVar `json:"additionalEnv,omitempty"`
	Debug         bool     `json:"debug,omitempty"`
	Periodic      bool     `json:"periodic,omitempty"`
	Schedule      string   `json:"schedule,omitempty"`
}

// OsintStatus defines the observed state of Osint
type OsintStatus struct {
	State        string `json:"state,omitempty"`
	JobName      string `json:"jobName,omitempty"`
	LastExecuted string `json:"lastExecuted,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Osint is the Schema for the osints API
type Osint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OsintSpec   `json:"spec,omitempty"`
	Status OsintStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OsintList contains a list of Osint
type OsintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Osint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Osint{}, &OsintList{})
}
