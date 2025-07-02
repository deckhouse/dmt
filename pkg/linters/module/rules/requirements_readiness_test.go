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

func TestReadinessProbesRequirementsCheck(t *testing.T) {
	helper := NewTestHelper(t)

	testCases := []TestCase{
		{
			Name: "readiness probes with module-sdk >= 0.3 - should fail",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, GoModWithModuleSDK03, MainGoWithReadiness)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements [readiness_probes]: Readiness probes usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.71.0"},
			Description:    "Если есть readiness probes (app.WithReadiness + module-sdk >= 0.3), то должен быть requirements с версией deckhouse не менее 1.71",
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
		{
			Name: "app.WithReadiness with module-sdk < 0.3 - should NOT fail",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, GoModWithModuleSDK, MainGoWithReadiness)
					return nil
				},
			},
			ExpectedErrors: []string{},
			Description:    "Если есть app.WithReadiness с module-sdk < 0.3, то НЕ должно быть ошибки",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(_ *testing.T) {
			helper.RunTestCase(&tc)
		})
	}
}

func TestReadinessProbesDetection(t *testing.T) {
	helper := NewTestHelper(t)
	modulePath := helper.CreateTempModule("test-readiness")

	// Test readiness probes detection
	helper.SetupGoHooks(modulePath, GoModWithModuleSDK03, MainGoWithReadiness)

	result := hasReadinessProbes(modulePath)
	assert.True(t, result, "Should detect readiness probes when module-sdk >= 0.3 and app.WithReadiness is used")

	// Test no readiness probes
	helper.SetupGoHooks(modulePath, GoModWithModuleSDK03, MainGoEmpty)

	result = hasReadinessProbes(modulePath)
	assert.False(t, result, "Should NOT detect readiness probes when app.WithReadiness is not used")

	// Test with older module-sdk
	helper.SetupGoHooks(modulePath, GoModWithModuleSDK, MainGoWithReadiness)

	result = hasReadinessProbes(modulePath)
	assert.False(t, result, "Should NOT detect readiness probes when module-sdk < 0.3")
}
