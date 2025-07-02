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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasGoHooksDetection(t *testing.T) {
	helper := NewTestHelper(t)
	modulePath := helper.CreateTempModule("test-go-hooks")

	tests := []struct {
		name          string
		goModContent  string
		mainGoContent string
		expected      bool
		description   string
	}{
		{
			name:          "module-sdk with app.Run() call",
			goModContent:  GoModWithModuleSDK,
			mainGoContent: MainGoWithAppRun,
			expected:      true,
			description:   "Should detect Go hooks when module-sdk is present and app.Run() is called",
		},
		{
			name:          "module-sdk with myApp.Run() call",
			goModContent:  GoModWithModuleSDK,
			mainGoContent: `package main\nfunc main() { myApp.Run() }`,
			expected:      true,
			description:   "Should detect Go hooks when module-sdk is present and myApp.Run() is called",
		},
		{
			name:          "module-sdk without Run() call",
			goModContent:  GoModWithModuleSDK,
			mainGoContent: MainGoEmpty,
			expected:      false,
			description:   "Should NOT detect Go hooks when module-sdk is present but no Run() is called",
		},
		{
			name:          "no module-sdk with app.Run() call",
			goModContent:  "module test",
			mainGoContent: MainGoWithAppRun,
			expected:      false,
			description:   "Should NOT detect Go hooks when no module-sdk is present even if app.Run() is called",
		},
		{
			name:          "no go.mod with app.Run() call",
			goModContent:  "",
			mainGoContent: MainGoWithAppRun,
			expected:      false,
			description:   "Should NOT detect Go hooks when no go.mod is present even if app.Run() is called",
		},
		{
			name:          "app.WithReadiness() call with module-sdk",
			goModContent:  GoModWithModuleSDK,
			mainGoContent: MainGoWithReadiness,
			expected:      false,
			description:   "Should NOT detect Go hooks when app.WithReadiness() is called instead of Run()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up previous test files
			if tt.goModContent != "" {
				helper.SetupGoHooks(modulePath, tt.goModContent, tt.mainGoContent)
			} else {
				helper.SetupGoHooks(modulePath, "", tt.mainGoContent)
			}

			result := hasGoHooks(modulePath)
			assert.Equal(t, tt.expected, result, "Test: %s\nExpected %v for go.mod: %s\nmain.go: %s",
				tt.description, tt.expected, tt.goModContent, tt.mainGoContent)
		})
	}
}

func TestGoHooksRequirementsCheck(t *testing.T) {
	helper := NewTestHelper(t)

	testCases := []TestCase{
		{
			Name: "go_hooks with module-sdk and app.Run - should fail",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, GoModWithModuleSDK, MainGoWithAppRun)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements: Go hooks usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.68.0"},
			Description:    "Если есть go_hooks (go.mod с module-sdk + app.Run), то должен быть requirements с версией deckhouse не менее 1.68",
		},
		{
			Name: "go.mod without module-sdk - should NOT fail",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, "module test", MainGoWithAppRun)
					return nil
				},
			},
			ExpectedErrors: []string{},
			Description:    "Если есть go.mod без module-sdk, то НЕ должно быть ошибки",
		},
		{
			Name: "module-sdk >= 0.3 without app.WithReadiness - should NOT fail",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, GoModWithModuleSDK03, MainGoEmpty)
					return nil
				},
			},
			ExpectedErrors: []string{},
			Description:    "Если есть только module-sdk >= 0.3 без app.WithReadiness, то НЕ должно быть ошибки",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(_ *testing.T) {
			helper.RunTestCase(&tc)
		})
	}
}
