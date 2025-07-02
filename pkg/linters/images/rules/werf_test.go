package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestNewWerfRule(t *testing.T) {
	rule := NewWerfRule()
	assert.NotNil(t, rule)
	assert.Equal(t, "werf", rule.GetName())
}

func TestWerfRule_LintWerfFile_ValidBaseImage(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	validWerfData := `
image: test-module/test-image
from: registry.deckhouse.io/base_images/ubuntu:22.04
final: true
`

	rule.LintWerfFile("test-module", validWerfData, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid base image")
}

func TestWerfRule_LintWerfFile_InvalidBaseImage(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	invalidWerfData := `
image: test-module/test-image
from: ubuntu:22.04
final: true
`

	rule.LintWerfFile("test-module", invalidWerfData, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid base image")
	assert.Contains(t, errorList.GetErrors()[0].Text, "`from:` parameter should be one of our BASE_DISTROLESS images")
}

func TestWerfRule_LintWerfFile_ArtifactDirective(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataWithArtifact := `
image: test-module/test-image
artifact: some-artifact
from: registry.deckhouse.io/base_images/ubuntu:22.04
final: true
`

	rule.LintWerfFile("test-module", werfDataWithArtifact, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for deprecated artifact directive")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Use `from:` or `fromImage:` and `final: false` directives instead of `artifact:`")
}

func TestWerfRule_LintWerfFile_NonFinalImage(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	nonFinalWerfData := `
image: test-module/test-image
from: ubuntu:22.04
final: false
`

	rule.LintWerfFile("test-module", nonFinalWerfData, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for non-final image")
}

func TestWerfRule_LintWerfFile_NoFromField(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataNoFrom := `
image: test-module/test-image
final: true
`

	rule.LintWerfFile("test-module", werfDataNoFrom, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when no 'from' field is present")
}

func TestWerfRule_LintWerfFile_EmptyFromField(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataEmptyFrom := `
image: test-module/test-image
from: ""
final: true
`

	rule.LintWerfFile("test-module", werfDataEmptyFrom, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when 'from' field is empty")
}

func TestWerfRule_LintWerfFile_WhitespaceFromField(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataWhitespaceFrom := `
image: test-module/test-image
from: "   "
final: true
`

	rule.LintWerfFile("test-module", werfDataWhitespaceFrom, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when 'from' field contains only whitespace")
}

func TestWerfRule_LintWerfFile_MultipleDocuments(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	multipleDocsData := `
---
image: test-module/test-image-1
from: registry.deckhouse.io/base_images/ubuntu:22.04
final: true
---
image: test-module/test-image-2
from: ubuntu:22.04
final: true
---
image: test-module/test-image-3
from: registry.deckhouse.io/base_images/alpine:3.18
final: true
`

	rule.LintWerfFile("test-module", multipleDocsData, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid base image in second document")

	errorListErrors := errorList.GetErrors()
	assert.Len(t, errorListErrors, 1, "Expected exactly one error")
	assert.Contains(t, errorListErrors[0].Text, "`from:` parameter should be one of our BASE_DISTROLESS images")
}

func TestWerfRule_LintWerfFile_InvalidYAML(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	invalidYAMLData := `
image: test-module/test-image
from: registry.deckhouse.io/base_images/ubuntu:22.04
final: true
  invalid: indentation: here
`

	rule.LintWerfFile("test-module", invalidYAMLData, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid YAML")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Invalid YAML document")
}

func TestWerfRule_LintWerfFile_EmptyFile(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	emptyData := ""

	rule.LintWerfFile("test-module", emptyData, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for empty file")
}

func TestWerfRule_LintWerfFile_WhitespaceOnlyFile(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	whitespaceData := "   \n\t  \n"

	rule.LintWerfFile("test-module", whitespaceData, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for whitespace-only file")
}

// TestSplitManifests indirectly through LintWerfFile
func TestWerfRule_LintWerfFile_SplitManifests(t *testing.T) {
	rule := NewWerfRule()

	// Test multiple documents
	multipleDocsData := `
---
image: test-module/test-image-1
from: registry.deckhouse.io/base_images/ubuntu:22.04
final: true
---
image: test-module/test-image-2
from: registry.deckhouse.io/base_images/alpine:3.18
final: true
`

	errorList := errors.NewLintRuleErrorsList()
	rule.LintWerfFile("test-module", multipleDocsData, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid multiple documents")
}

// TestIsWerfImagesCorrect indirectly through LintWerfFile
func TestWerfRule_LintWerfFile_ImageValidation(t *testing.T) {
	rule := NewWerfRule()

	// Test various image paths through the public interface
	testCases := []struct {
		name        string
		werfData    string
		expectError bool
	}{
		{
			name: "valid base_images path",
			werfData: `
image: test-module/test-image
from: registry.deckhouse.io/base_images/ubuntu:22.04
final: true
`,
			expectError: false,
		},
		{
			name: "invalid path without base_images",
			werfData: `
image: test-module/test-image
from: registry.deckhouse.io/other/ubuntu:22.04
final: true
`,
			expectError: true,
		},
		{
			name: "base_images not in second position",
			werfData: `
image: test-module/test-image
from: base_images/registry.deckhouse.io/ubuntu:22.04
final: true
`,
			expectError: true,
		},
		{
			name: "empty image path",
			werfData: `
image: test-module/test-image
from: ""
final: true
`,
			expectError: false,
		},
		{
			name: "single component path",
			werfData: `
image: test-module/test-image
from: ubuntu
final: true
`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			rule.LintWerfFile("test-module", tc.werfData, errorList)

			if tc.expectError {
				assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid image path")
			} else {
				assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid image path")
			}
		})
	}
}
