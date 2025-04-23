package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg/errors"
)

type mockModule struct {
	werfFile string
}

func (m *mockModule) GetWerfFile() string {
	return m.werfFile
}
func (*mockModule) GetPath() string {
	return "/mock/path"
}

func TestValidateWerfTemplates(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	// Mock module with valid Werf file
	mock := &mockModule{
		werfFile: `
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
	mockModuleInvalid := &mockModule{
		werfFile: `
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
# Missing stageDependencies

`}

	rule.ValidateWerfTemplates(mockModuleInvalid, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid Werf file")
	assert.Contains(t, errorList.GetErrors()[0].Text, "'git.stageDependencies' is required")
}

func TestCheckGitSection(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	// Valid manifest
	validManifests := []string{
		`
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`,
	}

	checkGitSection(validManifests, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid manifest")

	// Invalid manifest
	invalidManifests := []string{
		`
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  # Missing stageDependencies
`,
	}

	checkGitSection(invalidManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid manifest")
	assert.Contains(t, errorList.GetErrors()[0].Text, "'git.stageDependencies' is required")

	// Malformed YAML
	malformedManifests := []string{
		`
git:
  - stageDependencies: [build: "file1", "file2"]
`,
	}

	checkGitSection(malformedManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for malformed YAML")
	assert.Contains(t, errorList.GetErrors()[0].Text, "parsing Werf file, document 1 failed")
}
