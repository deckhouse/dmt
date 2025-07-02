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
)

func TestRequirementsRegistryAllChecks(t *testing.T) {
	helper := NewTestHelper(t)

	testCases := []TestCase{
		{
			Name: "stage without requirements",
			Setup: TestSetup{
				ModuleContent: StageModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, StageModuleContent)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements: Stage usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.68.0"},
		},
		{
			Name: "go hooks without requirements",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, GoModWithModuleSDK, MainGoWithAppRun)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements: Go hooks usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.68.0"},
		},
		{
			Name: "readiness probe + module-sdk >= 0.3 without requirements",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, GoModWithModuleSDK03, MainGoWithReadiness)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements: Readiness probes usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.71.0"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(_ *testing.T) {
			helper.RunTestCase(&tc)
		})
	}
}

func TestRequirementsLogicUserRequirements(t *testing.T) {
	helper := NewTestHelper(t)

	testCases := []TestCase{
		{
			Name: "stage without requirements - should fail",
			Setup: TestSetup{
				ModuleContent: StageModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, StageModuleContent)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements: Stage usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.68.0"},
			Description:    "Если в модуле есть stage, то должен быть requirements с версией deckhouse не менее 1.68",
		},
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
			Name: "readiness probes with module-sdk >= 0.3 - should fail",
			Setup: TestSetup{
				ModuleContent: ValidModuleContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, ValidModuleContent)
					helper.SetupGoHooks(path, GoModWithModuleSDK03, MainGoWithReadiness)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements: Readiness probes usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.71.0"},
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
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(_ *testing.T) {
			helper.RunTestCase(&tc)
		})
	}
}
