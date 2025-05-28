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
func TestCheckDefinitionFile_StageField(t *testing.T) {
	tempDir := t.TempDir()
	moduleFilePath := filepath.Join(tempDir, ModuleConfigFilename)

	// Test missing 'stage' field
	err := os.WriteFile(moduleFilePath, []byte(`
name: test-module
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	rule := NewDefinitionFileRule(false)
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for missing 'stage' field")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Field 'stage' is required")

	// Test invalid 'stage' value
	err = os.WriteFile(moduleFilePath, []byte(`
name: test-module
stage: InvalidStage
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid 'stage' value")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Field 'stage' is not one of the following values")

	// Test valid 'stage' value
	err = os.WriteFile(moduleFilePath, []byte(`
name: test-module
stage: Experimental
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid 'stage' value")
}

func TestCheckDefinitionFile_DescriptionsEnField(t *testing.T) {
	tempDir := t.TempDir()
	moduleFilePath := filepath.Join(tempDir, ModuleConfigFilename)

	// Test missing 'descriptions.en' field
	err := os.WriteFile(moduleFilePath, []byte(`
name: test-module
stage: Experimental
descriptions:
  ru: "Тестовое описание"
`), 0600)
	require.NoError(t, err)

	rule := NewDefinitionFileRule(false)
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected warning for missing 'descriptions.en' field")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Module `descriptions.en` field is required")

	// Test present 'descriptions.en' field
	err = os.WriteFile(moduleFilePath, []byte(`
name: test-module
stage: Experimental
descriptions:
  en: "Test description"
  ru: "Тестовое описание"
`), 0600)
	require.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no warnings for present 'descriptions.en' field")
}

func TestCheckDefinitionFile_FileErrors(t *testing.T) {
	tempDir := t.TempDir()
	moduleFilePath := filepath.Join(tempDir, ModuleConfigFilename)

	// Create a file with invalid YAML
	err := os.WriteFile(moduleFilePath, []byte(`:invalid_yaml`), 0600)
	require.NoError(t, err)

	rule := NewDefinitionFileRule(false)
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid YAML")
	assert.Contains(t, errorList.GetErrors()[0].Text, "Cannot parse file")

	// Remove read permissions to simulate read error
	err = os.WriteFile(moduleFilePath, []byte(`name: test-module`), 0000)
	require.NoError(t, err)
	defer func() {
		err := os.Chmod(moduleFilePath, 0600) // Restore permissions after test
		require.NoError(t, err)
	}()

	errorList = errors.NewLintRuleErrorsList()
	rule.CheckDefinitionFile(tempDir, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for unreadable file")
}

func TestCheckDefinitionFile_DisabledRule(t *testing.T) {
	tempDir := t.TempDir()
	moduleFilePath := filepath.Join(tempDir, ModuleConfigFilename)

	err := os.WriteFile(moduleFilePath, []byte(`
name: test-module
stage: Experimental
descriptions:
  en: "Test description"
`), 0600)
	require.NoError(t, err)

	rule := NewDefinitionFileRule(true)
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckDefinitionFile(tempDir, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors when rule is disabled")
}
