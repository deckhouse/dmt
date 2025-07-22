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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	// Default directory permissions for test files
	DefaultDirPerm = 0755
	// Default file permissions for test files
	DefaultFilePerm = 0600
)

// TestSetup represents the setup configuration for a test
type TestSetup struct {
	ModuleContent string
	SetupFiles    func(string) error
}

// TestCase represents a single test case
type TestCase struct {
	Name           string
	Setup          TestSetup
	ExpectedErrors []string
	Description    string
}

// TestHelper provides common testing utilities
type TestHelper struct {
	t *testing.T
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// CreateTempModule creates a temporary module directory for testing
func (h *TestHelper) CreateTempModule(name string) string {
	modulePath := filepath.Join(h.t.TempDir(), name)
	err := os.MkdirAll(modulePath, DefaultDirPerm)
	require.NoError(h.t, err, "failed to create module dir")
	return modulePath
}

// SetupModule creates module.yaml with given content
func (h *TestHelper) SetupModule(modulePath, content string) {
	err := os.WriteFile(filepath.Join(modulePath, ModuleConfigFilename), []byte(content), DefaultFilePerm)
	require.NoError(h.t, err, "failed to create module.yaml")
}

// SetupGoHooks creates hooks directory with go.mod and main.go files
func (h *TestHelper) SetupGoHooks(modulePath, goModContent, mainGoContent string) {
	hooksDir := filepath.Join(modulePath, "hooks")
	err := os.MkdirAll(hooksDir, DefaultDirPerm)
	require.NoError(h.t, err, "failed to create hooks dir")

	if goModContent != "" {
		err = os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(goModContent), DefaultFilePerm)
		require.NoError(h.t, err, "failed to create go.mod")
	}

	if mainGoContent != "" {
		err = os.WriteFile(filepath.Join(hooksDir, "main.go"), []byte(mainGoContent), DefaultFilePerm)
		require.NoError(h.t, err, "failed to create main.go")
	}
}

// RunRequirementsCheck runs the requirements check and returns the error list
func RunRequirementsCheck(modulePath string) *errors.LintRuleErrorsList {
	rule := NewRequirementsRule()
	errorList := errors.NewLintRuleErrorsList()
	rule.CheckRequirements(modulePath, errorList)
	return errorList
}

// AssertErrors asserts that the error list contains the expected errors
func (h *TestHelper) AssertErrors(errorList *errors.LintRuleErrorsList, expectedErrors []string) {
	if len(expectedErrors) == 0 {
		assert.False(h.t, errorList.ContainsErrors(), "Expected no errors but got: %v", errorList.GetErrors())
	} else {
		assert.True(h.t, errorList.ContainsErrors(), "Expected errors but got none")
		errs := errorList.GetErrors()
		assert.Len(h.t, errs, len(expectedErrors), "Expected %d errors, got %d", len(expectedErrors), len(errs))

		for i, expectedError := range expectedErrors {
			if i < len(errs) {
				assert.Contains(h.t, errs[i].Text, expectedError, "Error %d should contain '%s'", i, expectedError)
			}
		}
	}
}

// RunTestCase runs a single test case with the given setup
func (h *TestHelper) RunTestCase(tc *TestCase) {
	modulePath := h.CreateTempModule(tc.Name)

	if tc.Setup.SetupFiles != nil {
		err := tc.Setup.SetupFiles(modulePath)
		require.NoError(h.t, err)
	}

	errorList := RunRequirementsCheck(modulePath)
	h.AssertErrors(errorList, tc.ExpectedErrors)
}

// Common test data constants
const (
	ValidModuleContent = `name: test-module
namespace: test`

	StageModuleContent = `name: test-module
namespace: test
stage: "General Availability"`

	StageWithRequirementsContent = `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0"`

	GoModWithModuleSDK = `module test
require github.com/deckhouse/module-sdk v0.1.0`

	GoModWithModuleSDK03 = `module test
require github.com/deckhouse/module-sdk v0.3.0`

	MainGoWithAppRun = `package main
func main() { app.Run() }`

	MainGoWithReadiness = `package main
func main() { app.WithReadiness() }`

	MainGoEmpty = `package main
func main() { }`
)
