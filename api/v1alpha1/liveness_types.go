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

// LivenessSpec defines the desired state of Liveness
type LivenessSpec struct {
	// Target is the primary target for liveness check (IP, CIDR, or hostname). Can be either a direct target or a reference to a TargetPool name.
	// +optional
	Target string `json:"target,omitempty"`

	// TargetPool is the name of a TargetPool resource to reference for target and port information
	// +optional
	TargetPool string `json:"targetPool,omitempty"`

	// Targets are additional targets for liveness checks
	// +optional
	Targets []string `json:"targets,omitempty"`

	// Tool is the liveness check tool to use (ex ping)
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

	// Debug enables debug mode (output to stdout instead of log file)
	// +optional
	Debug bool `json:"debug,omitempty"`

	// Category is a label to categorize the liveness check (e.g., 'critical-targets')
	// +optional
	Category string `json:"category,omitempty"`
}

// LivenessStatus defines the observed state of Liveness
type LivenessStatus struct {
	// LastExecuted is the timestamp of last execution
	// +optional
	LastExecuted string `json:"lastExecuted,omitempty"`

	// LastResult indicates if the target was reachable
	// +optional
	LastResult bool `json:"lastResult,omitempty"`

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

	// ResolvedTarget is the resolved target after TargetPool reference is applied
	// +optional
	ResolvedTarget string `json:"resolvedTarget,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=live,singular=liveness
//+kubebuilder:printcolumn:name="Tool",type=string,JSONPath=`.spec.tool`
//+kubebuilder:printcolumn:name="TargetPool",type=string,JSONPath=`.spec.targetPool`
//+kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.status.resolvedTarget`
//+kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
//+kubebuilder:printcolumn:name="Periodic",type=boolean,JSONPath=`.spec.periodic`
//+kubebuilder:printcolumn:name="LastResult",type=boolean,JSONPath=`.status.lastResult`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Liveness is the Schema for the livenesses API
type Liveness struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LivenessSpec   `json:"spec,omitempty"`
	Status LivenessStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LivenessList contains a list of Liveness
type LivenessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Liveness `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Liveness{}, &LivenessList{})
}
