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

func TestStageRequirementsCheck(t *testing.T) {
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
			ExpectedErrors: []string{"requirements [stage]: Stage usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.68.0"},
			Description:    "Если в модуле есть stage, то должен быть requirements с версией deckhouse не менее 1.68",
		},
		{
			Name: "stage with valid requirements - should pass",
			Setup: TestSetup{
				ModuleContent: StageWithRequirementsContent,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, StageWithRequirementsContent)
					return nil
				},
			},
			ExpectedErrors: []string{},
			Description:    "Если в модуле есть stage с корректными requirements, то не должно быть ошибки",
		},
		{
			Name: "stage with requirements below minimum - should fail",
			Setup: TestSetup{
				ModuleContent: `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.67.0"`,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.67.0"`)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements [stage]: Stage usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.68.0 (currently: 1.67.0)"},
			Description:    "Если в модуле есть stage с requirements ниже минимальной версии, то должна быть ошибка",
		},
		{
			Name: "stage with invalid deckhouse constraint - should fail",
			Setup: TestSetup{
				ModuleContent: `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "invalid-constraint"`,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "invalid-constraint"`)
					return nil
				},
			},
			ExpectedErrors: []string{"requirements [stage]: invalid deckhouse version constraint: invalid-constraint"},
			Description:    "Если в модуле есть stage с некорректным constraint, то должна быть ошибка",
		},
		{
			Name: "stage with complex valid constraint - should pass",
			Setup: TestSetup{
				ModuleContent: `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0, < 2.0.0"`,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0, < 2.0.0"`)
					return nil
				},
			},
			ExpectedErrors: []string{},
			Description:    "Если в модуле есть stage с комплексным корректным constraint, то не должно быть ошибки",
		},
		{
			Name: "stage with exact version constraint - should pass",
			Setup: TestSetup{
				ModuleContent: `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "= 1.68.0"`,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "= 1.68.0"`)
					return nil
				},
			},
			ExpectedErrors: []string{},
			Description:    "Если в модуле есть stage с точной версией, то не должно быть ошибки",
		},
		{
			Name: "stage with greater than constraint - should pass",
			Setup: TestSetup{
				ModuleContent: `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "> 1.68.0"`,
				SetupFiles: func(path string) error {
					helper.SetupModule(path, `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "> 1.68.0"`)
					return nil
				},
			},
			ExpectedErrors: []string{},
			Description:    "Если в модуле есть stage с greater than constraint, то не должно быть ошибки",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(_ *testing.T) {
			helper.RunTestCase(&tc)
		})
	}
}
