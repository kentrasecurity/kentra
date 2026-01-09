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

// EnumerationAttackTest implements AttackTest for Enumeration operations
type EnumerationAttackTest struct{}

func (e *EnumerationAttackTest) GetName() string {
	return "enumeration"
}

func (e *EnumerationAttackTest) GetSampleFiles(projectDir string) []string {
	return []string{
		filepath.Join(projectDir, "config/samples/targetpools/kttack_v1alpha1_targetpool_nmap_ports.yaml"),
		filepath.Join(projectDir, "config/samples/attacks/kttack_v1alpha1_enumeration_nmap_scan_ports.yaml"),
	}
}

func (e *EnumerationAttackTest) GetResourceName() string {
	return "nmap-scan-ports"
}

func (e *EnumerationAttackTest) GetResourceType() string {
	return "enumeration"
}

func (e *EnumerationAttackTest) GetAppLabel() string {
	return "enumeration"
}

func (e *EnumerationAttackTest) ValidateSpecificBehavior(namespace string) error {
	// Add Enumeration-specific validations here if needed
	// For example: verify that TargetPool was resolved correctly
	return nil
}
