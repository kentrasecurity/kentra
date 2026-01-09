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

package e2e

import (
	"path/filepath"
)

// LivenessAttackTest implements AttackTest for Liveness operations
type LivenessAttackTest struct{}

func (l *LivenessAttackTest) GetName() string {
	return "liveness"
}

func (l *LivenessAttackTest) GetSampleFiles(projectDir string) []string {
	return []string{
		filepath.Join(projectDir, "config/samples/targetpools/kttack_v1alpha1_targetpool_osint.yaml"),
		// Add your liveness sample file here when you create it
		// filepath.Join(projectDir, "config/samples/attacks/security_v1alpha1_liveness_ping.yaml"),
	}
}

func (l *LivenessAttackTest) GetResourceName() string {
	return "ping-sample" // Adjust based on your actual sample
}

func (l *LivenessAttackTest) GetResourceType() string {
	return "liveness"
}

func (l *LivenessAttackTest) GetAppLabel() string {
	return "liveness"
}

func (l *LivenessAttackTest) ValidateSpecificBehavior(namespace string) error {
	// Add Liveness-specific validations here if needed
	// For example: verify that ping succeeded
	return nil
}
