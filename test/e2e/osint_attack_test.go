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

// OsintAttackTest implements AttackTest for OSINT operations
type OsintAttackTest struct{}

func (o *OsintAttackTest) GetName() string {
	return "osint"
}

func (o *OsintAttackTest) GetSampleFiles(projectDir string) []string {
	return []string{
		filepath.Join(projectDir, "config/samples/assetpools/assetpool-sherlock.yaml"),
		filepath.Join(projectDir, "config/samples/attacks/security_v1alpha1_osint_sherlock_with_assets.yaml"),
	}
}

func (o *OsintAttackTest) GetResourceName() string {
	return "osint-sample"
}

func (o *OsintAttackTest) GetResourceType() string {
	return "osint"
}

func (o *OsintAttackTest) GetAppLabel() string {
	return "osint"
}

func (o *OsintAttackTest) ValidateSpecificBehavior(namespace string) error {
	// Add OSINT-specific validations here if needed
	// For example: verify that AssetPool was resolved correctly
	return nil
}
