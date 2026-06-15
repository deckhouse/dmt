package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestValidateWerfTemplates(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "test-image")
	if err := os.WriteFile(filePath, []byte(`image: {{ include "helm_lib_module_image" . "mock-module/test-image" }}`), 0644); err != nil {
		t.Fatal(err)
	}

	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	// Mock module with valid Werf file
	mc := minimock.NewController(t)

	mock := mocks.NewModuleMock(mc)
	mock.GetPathMock.Return("/mock/path")
	mock.GetWerfFileMock.Return(`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`)

	rule.ValidateWerfTemplates(mock, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid Werf file")

	errorList = errors.NewLintRuleErrorsList()
	// Mock module with invalid Werf file (image name contains an underscore)
	mockModuleWerfInvalid := mocks.NewModuleMock(mc)
	mockModuleWerfInvalid.GetPathMock.Return("/mock/path")
	mockModuleWerfInvalid.GetWerfFileMock.Return(`
image: mock-module/test_image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
`)
	rule.ValidateWerfTemplates(mockModuleWerfInvalid, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid Werf file")
	assert.Contains(t, errorList.GetErrors()[0].Text, "must not contain underscores")
}

func TestCheckUnderscoredImages(t *testing.T) {
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

	checkUnderscoredImages(validManifests, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid manifest")

	// Invalid manifest
	invalidManifests := []string{
		`
image: mock-module/test-image_invalid
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`,
	}

	checkUnderscoredImages(invalidManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid manifest")
	assert.Contains(t, errorList.GetErrors()[0].Text, "must not contain underscores")
}
