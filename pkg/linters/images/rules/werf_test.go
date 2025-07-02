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
fromImage: base/disstroless
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
fromImage: disstroless
final: true
`

	rule.LintWerfFile("test-module", invalidWerfData, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid base image")
	assert.Contains(t, errorList.GetErrors()[0].Text, "`fromImage:` parameter should be one of our `base` images")
}

func TestWerfRule_LintWerfFile_ArtifactDirective(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataWithArtifact := `
image: test-module/test-image
artifact: some-artifact
fromImage: base/disstroless
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
fromImage: disstroless
final: false
`

	rule.LintWerfFile("test-module", nonFinalWerfData, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for non-final image")
}

func TestWerfRule_LintWerfFile_NoFromImageField(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataNoFromImage := `
image: test-module/test-image
final: true
`

	rule.LintWerfFile("test-module", werfDataNoFromImage, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when no 'fromImage' field is present")
}

func TestWerfRule_LintWerfFile_EmptyFromImageField(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataEmptyFromImage := `
image: test-module/test-image
fromImage: ""
final: true
`

	rule.LintWerfFile("test-module", werfDataEmptyFromImage, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when 'fromImage' field is empty")
}

func TestWerfRule_LintWerfFile_WhitespaceFromImageField(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	werfDataWhitespaceFromImage := `
image: test-module/test-image
fromImage: "   "
final: true
`

	rule.LintWerfFile("test-module", werfDataWhitespaceFromImage, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when 'fromImage' field contains only whitespace")
}

func TestWerfRule_LintWerfFile_MultipleDocuments(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	multipleDocsData := `
---
image: test-module/test-image-1
fromImage: base/disstroless
final: true
---
image: test-module/test-image-2
fromImage: disstroless
final: true
---
image: test-module/test-image-3
fromImage: base/alpine:3.18
final: true
`

	rule.LintWerfFile("test-module", multipleDocsData, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid base image in second document")

	errorListErrors := errorList.GetErrors()
	assert.Len(t, errorListErrors, 1, "Expected exactly one error")
	assert.Contains(t, errorListErrors[0].Text, "`fromImage:` parameter should be one of our `base` images")
}

func TestWerfRule_LintWerfFile_InvalidYAML(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	invalidYAMLData := `
image: test-module/test-image
fromImage: base/disstroless
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
fromImage: base/disstroless
final: true
---
image: test-module/test-image-2
fromImage: base/alpine:3.18
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
			name: "valid base path",
			werfData: `
image: test-module/test-image
fromImage: base/disstroless
final: true
`,
			expectError: false,
		},
		{
			name: "valid common path",
			werfData: `
image: test-module/test-image
fromImage: common/alpine:3.18
final: true
`,
			expectError: false,
		},
		{
			name: "invalid path without base or common",
			werfData: `
image: test-module/test-image
fromImage: other/disstroless
final: true
`,
			expectError: true,
		},
		{
			name: "empty image path",
			werfData: `
image: test-module/test-image
fromImage: ""
final: true
`,
			expectError: false,
		},
		{
			name: "single component path",
			werfData: `
image: test-module/test-image
fromImage: ubuntu
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

func TestWerfRule_LintWerfFile_ImageSpecConfigUser(t *testing.T) {
	rule := NewWerfRule()

	testCases := []struct {
		name        string
		werfData    string
		expectError bool
		errorText   string
	}{
		{
			name: "empty imageSpec.config.user",
			werfData: `
image: test-module/test-image
fromImage: base/disstroless
final: true
imageSpec:
  config:
    user: ""
`,
			expectError: false,
		},
		{
			name: "no imageSpec.config.user field",
			werfData: `
image: test-module/test-image
fromImage: base/disstroless
final: true
`,
			expectError: false,
		},
		{
			name: "non-empty imageSpec.config.user",
			werfData: `
image: test-module/test-image
fromImage: base/disstroless
final: true
imageSpec:
  config:
    user: "1000:1000"
`,
			expectError: true,
			errorText:   "`imageSpec.config.user:` parameter should be empty",
		},
		{
			name: "whitespace-only imageSpec.config.user",
			werfData: `
image: test-module/test-image
fromImage: base/disstroless
final: true
imageSpec:
  config:
    user: "   "
`,
			expectError: true,
			errorText:   "`imageSpec.config.user:` parameter should be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			rule.LintWerfFile("test-module", tc.werfData, errorList)

			if tc.expectError {
				assert.True(t, errorList.ContainsErrors(), "Expected errors for non-empty imageSpec.config.user")
				if tc.errorText != "" {
					assert.Contains(t, errorList.GetErrors()[0].Text, tc.errorText)
				}
			} else {
				assert.False(t, errorList.ContainsErrors(), "Expected no errors for empty imageSpec.config.user")
			}
		})
	}
}

func TestWerfRule_LintWerfFile_MultipleErrors(t *testing.T) {
	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	// Test case with both invalid fromImage and non-empty imageSpec.config.user
	werfDataWithMultipleIssues := `
image: test-module/test-image
fromImage: invalid/path
final: true
imageSpec:
  config:
    user: "1000:1000"
`

	rule.LintWerfFile("test-module", werfDataWithMultipleIssues, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for multiple issues")

	errs := errorList.GetErrors()
	assert.Len(t, errs, 2, "Expected exactly two errors")

	// Check that both errors are present
	errorTexts := []string{errs[0].Text, errs[1].Text}
	assert.Contains(t, errorTexts, "`fromImage:` parameter should be one of our `base` images")
	assert.Contains(t, errorTexts, "`imageSpec.config.user:` parameter should be empty")
}
