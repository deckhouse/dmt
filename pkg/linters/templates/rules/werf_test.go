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

// writeImageWerfFile writes content to <moduleDir>/images/<image>/werf.inc.yaml.
func writeImageWerfFile(t *testing.T, moduleDir, image, content string) {
	t.Helper()

	dir := filepath.Join(moduleDir, "images", image)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "werf.inc.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWerfTemplates(t *testing.T) {
	mc := minimock.NewController(t)

	// Module with a valid werf.inc.yaml (no underscores in image name).
	validDir := t.TempDir()
	writeImageWerfFile(t, validDir, "test-image", `
image: {{ .ModuleNamePrefix }}{{ .ImageName }}
git:
- add: /src
  to: /src
`)

	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	validModule := mocks.NewModuleMock(mc)
	validModule.GetPathMock.Return(validDir)

	rule.ValidateWerfTemplates(validModule, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid Werf file")

	// Module with an invalid werf.inc.yaml (image name contains an underscore).
	invalidDir := t.TempDir()
	writeImageWerfFile(t, invalidDir, "test-image", `
image: {{ .ModuleNamePrefix }}_{{ .ImageName }}_
fromImage: scratch
`)

	errorList = errors.NewLintRuleErrorsList()

	invalidModule := mocks.NewModuleMock(mc)
	invalidModule.GetPathMock.Return(invalidDir)

	rule.ValidateWerfTemplates(invalidModule, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid Werf file")
	assert.Contains(t, errorList.GetErrors()[0].Text, "must not contain underscores")
	assert.Equal(t, filepath.ToSlash(filepath.Join("images", "test-image", "werf.inc.yaml")),
		errorList.GetErrors()[0].FilePath, "Expected a relative file path that includes the werf file name")
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

	checkUnderscoredImages("images/test-image/werf.inc.yaml", validManifests, errorList)
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

	checkUnderscoredImages("images/test-image/werf.inc.yaml", invalidManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid manifest")
	assert.Contains(t, errorList.GetErrors()[0].Text, "must not contain underscores")
}
