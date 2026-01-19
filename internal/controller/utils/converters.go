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

package utils

import (
	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ConvertEnvVars converts CRD EnvVar to Kubernetes EnvVar
func ConvertEnvVars(crdEnvVars []securityv1alpha1.EnvVar) []corev1.EnvVar {
	if len(crdEnvVars) == 0 {
		return []corev1.EnvVar{}
	}

	k8sEnvVars := make([]corev1.EnvVar, len(crdEnvVars))
	for i, ev := range crdEnvVars {
		k8sEnvVars[i] = corev1.EnvVar{
			Name:  ev.Name,
			Value: ev.Value,
		}
	}
	return k8sEnvVars
}
