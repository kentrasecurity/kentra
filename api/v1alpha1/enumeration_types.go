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

// EnvVar represents an environment variable
type EnvVar struct {
	// Name of the environment variable
	Name string `json:"name"`
	// Value of the environment variable
	Value string `json:"value"`
}

// EnumerationSpec defines the desired state of Enumeration
type EnumerationSpec struct {
	// Target is the IP, CIDR, or hostname to enumerate
	// +kubebuilder:validation:Required
	Target string `json:"target"`

	// Tool is the enumeration tool to use (ex nmap)
	// +kubebuilder:validation:Required
	Tool string `json:"tool"`

	// FileName contains the output file names
	// +optional
	FileName []string `json:"fileName,omitempty"`

	// Periodic indicates if this should run on a schedule
	// +optional
	Periodic bool `json:"periodic,omitempty"`

	// Schedule is the cron schedule (only used if Periodic is true)
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// HTTPProxy is the HTTP proxy URL to use
	// +optional
	HTTPProxy string `json:"http_proxy,omitempty"`

	// Capabilities are additional capabilities required by the tool
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`

	// AdditionalEnv contains additional environment variables
	// +optional
	AdditionalEnv []EnvVar `json:"additional_env,omitempty"`

	// Args contains additional arguments for the tool
	// +optional
	Args []string `json:"args,omitempty"`

	// Debug if true, logs are written to stdout instead of emptydir volume
	// +optional
	Debug bool `json:"debug,omitempty"`
}

// EnumerationStatus defines the observed state of Enumeration
type EnumerationStatus struct {
	// LastExecuted is the timestamp of last execution
	// +optional
	LastExecuted string `json:"lastExecuted,omitempty"`

	// JobName is the name of the created job or cronjob
	// +optional
	JobName string `json:"jobName,omitempty"`

	// State represents the current state
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	State string `json:"state,omitempty"`

	// ResultsLocation is the path where results are stored
	// +optional
	ResultsLocation string `json:"resultsLocation,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=enum,singular=enumeration
//+kubebuilder:printcolumn:name="Tool",type=string,JSONPath=`.spec.tool`
//+kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target`
//+kubebuilder:printcolumn:name="Periodic",type=boolean,JSONPath=`.spec.periodic`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Enumeration is the Schema for the enumerations API
type Enumeration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnumerationSpec   `json:"spec,omitempty"`
	Status EnumerationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EnumerationList contains a list of Enumeration
type EnumerationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Enumeration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Enumeration{}, &EnumerationList{})
}
