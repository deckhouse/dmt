package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestCheckDefinitionFile(t *testing.T) {
	tempDir := t.TempDir()
	moduleFilePath := filepath.Join(tempDir, ModuleConfigFilename)

	err := os.WriteFile(moduleFilePath, []byte(`
name: test-module
stage: Experimental
descriptions:
  en: "Test description"
requirements:
  deckhouse: ">=1.0.0"
  kubernetes: ">=1.20.0"
`), 0600)
	require.NoError(t, err)

	rule := NewDefinitionFileRule(false)
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid module.yaml")

	_ = os.Remove(moduleFilePath)
	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when module.yaml is missing")
}

func TestCheckDefinitionFile_NameField(t *testing.T) {
	tempDir := t.TempDir()
	moduleFilePath := filepath.Join(tempDir, ModuleConfigFilename)

	// Test missing 'name' field
	err := os.WriteFile(moduleFilePath, []byte(`
stage: Experimental
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	rule := NewDefinitionFileRule(false)
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for missing 'name' field")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Field 'name' is required")

	// Test 'name' field exceeding 64 characters
	err = os.WriteFile(moduleFilePath, []byte(`
name: "this-is-a-very-long-module-name-that-exceeds-the-sixty-four-character-limit"
stage: Experimental
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for 'name' field exceeding 64 characters")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Field 'name' must not exceed 64 characters")

	// Test valid 'name' field
	err = os.WriteFile(moduleFilePath, []byte(`
name: "valid-module-name"
stage: Experimental
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid 'name' field")
}

func TestValidateRequirements(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	requirements := ModuleRequirements{
		ModulePlatformRequirements: ModulePlatformRequirements{
			Deckhouse:  ">=1.0.0",
			Kubernetes: ">=1.20.0",
		},
		ParentModules: map[string]string{
			"parent-module": ">=2.0.0",
		},
	}
	requirements.validateRequirements(errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid requirements")

	invalidRequirements := ModuleRequirements{
		ModulePlatformRequirements: ModulePlatformRequirements{
			Deckhouse:  "invalid-version",
			Kubernetes: ">=1.20.0",
		},
		ParentModules: map[string]string{
			"parent-module": "invalid-version",
		},
	}
	errorList = errors.NewLintRuleErrorsList()
	invalidRequirements.validateRequirements(errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid requirements")
}

func TestCheckDefinitionFile_CriticalAndWeight(t *testing.T) {
	tempDir := t.TempDir()
	moduleFilePath := filepath.Join(tempDir, ModuleConfigFilename)

	// 1. Critical: true, Weight: 0 (should produce error)
	err := os.WriteFile(moduleFilePath, []byte(`
name: test-critical
critical: true
weight: 0
stage: Experimental
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	rule := NewDefinitionFileRule(false)
	errorList := errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected error for critical module with zero weight")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Field 'weight' must be zero for critical modules")

	// 2. Critical: true, Weight: 10 (should not produce error)
	err = os.WriteFile(moduleFilePath, []byte(`
name: test-critical
critical: true
weight: 10
stage: Experimental
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no error for critical module with non-zero weight")

	// 3. Invalid stage value (should produce error)
	err = os.WriteFile(moduleFilePath, []byte(`
name: test-stage
stage: InvalidStage
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected error for invalid stage value")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Field 'stage' is not one of the following values")

	// 4. Invalid requirements (invalid deckhouse version, should produce error)
	err = os.WriteFile(moduleFilePath, []byte(`
name: test-req
stage: Experimental
descriptions:
  en: "Test description"
requirements:
  deckhouse: "invalid-version"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected error for invalid deckhouse version in requirements")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Invalid Deckhouse version requirement")

	// 5. Missing descriptions.en (should produce warning)
	err = os.WriteFile(moduleFilePath, []byte(`
name: test-desc
stage: Experimental
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	warnings := []string{}
	for _, e := range errorList.GetErrors() {
		if e.Level == 0 { // pkg.Warn
			warnings = append(warnings, e.Text)
		}
	}
	assert.NotEmpty(t, warnings, "Expected warning for missing descriptions.en")
	found := false
	for _, w := range warnings {
		if w == "Module `descriptions.en` field is required" {
			found = true
		}
	}
	assert.True(t, found, "Expected warning text for missing descriptions.en")
}
