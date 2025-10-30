/*
Copyright 2025 Flant JSC

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

package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestDeckhouseVersionRequirementRule_CheckDeckhouseVersionRequirement(t *testing.T) {
	tests := []struct {
		name           string
		setupFiles     func(t *testing.T, tempDir string)
		expectedErrors []string
	}{
		{
			name: "Module-SDK < 1.3 should not trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create module.yaml with old Deckhouse version
				moduleYaml := `name: test-module
requirements:
  deckhouse: ">= 1.60.0"`
				require.NoError(t, os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte(moduleYaml), 0600))

				// Create go.mod with old Module-SDK version
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.2.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{},
		},
		{
			name: "Module-SDK >= 1.3 with Deckhouse >= 1.71 should not trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create module.yaml with correct Deckhouse version
				moduleYaml := `name: test-module
requirements:
  deckhouse: ">= 1.71.0"`
				require.NoError(t, os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte(moduleYaml), 0600))

				// Create go.mod with new Module-SDK version
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.3.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{},
		},
		{
			name: "Module-SDK >= 1.3 with Deckhouse < 1.71 should trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create module.yaml with old Deckhouse version
				moduleYaml := `name: test-module
requirements:
  deckhouse: ">= 1.60.0, < 1.71.0"`
				require.NoError(t, os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte(moduleYaml), 0600))

				// Create go.mod with new Module-SDK version
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.3.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{
				"Module-SDK version >= 1.3.0 requires Deckhouse version >= 1.71.0, but current constraint '>= 1.60.0, < 1.71.0' allows minimum 1.60.0",
			},
		},
		{
			name: "Module-SDK >= 1.3 without module.yaml should trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create go.mod with new Module-SDK version but no module.yaml
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.3.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{
				"Module-SDK version >= 1.3.0 requires Deckhouse version >= 1.71.0, but module.yaml not found",
			},
		},
		{
			name: "Module-SDK >= 1.3 with module.yaml but no requirements should trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create module.yaml without requirements
				moduleYaml := `name: test-module`
				require.NoError(t, os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte(moduleYaml), 0600))

				// Create go.mod with new Module-SDK version
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.3.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{
				"Module-SDK version >= 1.3.0 requires Deckhouse version >= 1.71.0, but requirements.deckhouse is not specified",
			},
		},
		{
			name: "Module-SDK >= 1.3 with invalid Deckhouse version constraint should trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create module.yaml with invalid Deckhouse version
				moduleYaml := `name: test-module
requirements:
  deckhouse: "invalid-version"`
				require.NoError(t, os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte(moduleYaml), 0600))

				// Create go.mod with new Module-SDK version
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.3.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{
				"Invalid Deckhouse version constraint 'invalid-version':",
			},
		},
		{
			name: "Module-SDK >= 1.3 with exact Deckhouse version 1.71 should not trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create module.yaml with exact Deckhouse version
				moduleYaml := `name: test-module
requirements:
  deckhouse: "= 1.71.0"`
				require.NoError(t, os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte(moduleYaml), 0600))

				// Create go.mod with new Module-SDK version
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.3.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{},
		},
		{
			name: "Module-SDK >= 1.3 with range Deckhouse version should not trigger error",
			setupFiles: func(t *testing.T, tempDir string) {
				// Create module.yaml with range Deckhouse version
				moduleYaml := `name: test-module
requirements:
  deckhouse: ">= 1.71.0, < 2.0.0"`
				require.NoError(t, os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte(moduleYaml), 0600))

				// Create go.mod with new Module-SDK version
				hooksDir := filepath.Join(tempDir, "hooks")
				require.NoError(t, os.MkdirAll(hooksDir, 0755))
				goMod := `module test-module

go 1.21

require github.com/deckhouse/module-sdk v1.3.0`
				require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goMod), 0600))
			},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()

			// Setup test files
			tt.setupFiles(t, tempDir)

			// Create test object
			obj := &unstructured.Unstructured{}
			obj.SetAPIVersion("apps/v1")
			obj.SetKind("Deployment")
			obj.SetName("test-deployment")
			obj.SetNamespace("default")

			// Create templates directory
			templatesDir := filepath.Join(tempDir, "templates")
			require.NoError(t, os.MkdirAll(templatesDir, 0755))

			storeObj := &storage.StoreObject{
				Unstructured: *obj,
				AbsPath:      filepath.Join(templatesDir, "deployment.yaml"),
			}
			// Set the shortPath using reflection since it's not exported
			// We'll use reflection to set the unexported field
			v := reflect.ValueOf(storeObj).Elem()
			shortPathField := v.FieldByName("shortPath")
			if shortPathField.CanSet() {
				shortPathField.SetString("templates/deployment.yaml")
			}

			// Create error list
			errorList := errors.NewLintRuleErrorsList()

			// Create rule and run test
			rule := NewDeckhouseVersionRequirementRule()
			rule.CheckDeckhouseVersionRequirement(*storeObj, errorList)

			// Check results
			errorListErrors := errorList.GetErrors()
			assert.Len(t, errorListErrors, len(tt.expectedErrors), "Expected %d errors, got %d", len(tt.expectedErrors), len(errorListErrors))

			for i, expectedError := range tt.expectedErrors {
				if i < len(errorListErrors) {
					assert.Contains(t, errorListErrors[i].Text, expectedError, "Error %d should contain '%s'", i, expectedError)
				}
			}
		})
	}
}
