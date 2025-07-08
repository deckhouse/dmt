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

	"github.com/deckhouse/dmt/pkg/errors"
)

type mockModuleWerf struct {
	werfFile string
}

func (m *mockModuleWerf) GetWerfFile() string {
	return m.werfFile
}

func (*mockModuleWerf) GetPath() string {
	return "/mock/path"
}

func (*mockModuleWerf) GetName() string {
	return "mock-module"
}

func TestValidateWerfTemplates(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	// Mock module with valid Werf file
	mock := &mockModuleWerf{
		werfFile: `
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`}

	rule.ValidateWerfTemplates(mock, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid Werf file")

	errorList = errors.NewLintRuleErrorsList()
	// Mock module with invalid Werf file
	mockModuleWerfInvalid := &mockModuleWerf{
		werfFile: `
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
# Missing stageDependencies

`}

	rule.ValidateWerfTemplates(mockModuleWerfInvalid, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid Werf file")
	assert.Contains(t, errorList.GetErrors()[0].Text, "'git.stageDependencies' is required")
}

func TestCheckGitSection(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	// Valid manifest
	validManifests := []string{
		`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`,
	}

	checkGitSection("mock-module", validManifests, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid manifest")

	// Invalid manifest
	invalidManifests := []string{
		`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  # Missing stageDependencies
`,
	}

	checkGitSection("mock-module", invalidManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid manifest")
	assert.Contains(t, errorList.GetErrors()[0].Text, "'git.stageDependencies' is required")

	// Malformed YAML
	malformedManifests := []string{
		`
image: mock-module/test-image
git:
  - stageDependencies: [build: "file1", "file2"]
`,
	}

	checkGitSection("mock-module", malformedManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for malformed YAML")
	assert.Contains(t, errorList.GetErrors()[0].Text, "parsing Werf file, document 1 (image: mock-module/test-image) failed")
}
