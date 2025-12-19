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

package conversions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/test"
)

func TestTester_Type(t *testing.T) {
	tester := NewTester()
	assert.Equal(t, test.TestTypeConversions, tester.Type())
}

func TestTester_CanRun(t *testing.T) {
	// Create a temp directory with proper structure
	tmpDir := t.TempDir()

	tester := NewTester()

	// Without test file - should return false
	assert.False(t, tester.CanRun(tmpDir))

	// Create conversions folder and test file
	conversionsDir := filepath.Join(tmpDir, "openapi", "conversions")
	err := os.MkdirAll(conversionsDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(conversionsDir, testCasesFile)
	err = os.WriteFile(testFile, []byte("cases: []"), 0600)
	require.NoError(t, err)

	// With test file - should return true
	assert.True(t, tester.CanRun(tmpDir))
}

func TestTester_Run(t *testing.T) {
	// Create a temp directory with full test setup
	tmpDir := t.TempDir()
	conversionsDir := filepath.Join(tmpDir, "openapi", "conversions")
	err := os.MkdirAll(conversionsDir, 0755)
	require.NoError(t, err)

	// Create conversion file v2.yaml
	conversionContent := `version: 2
description:
  en: "Test conversion"
  ru: "Тестовая конверсия"
conversions:
  - del(.removeMe)
`
	err = os.WriteFile(filepath.Join(conversionsDir, "v2.yaml"), []byte(conversionContent), 0600)
	require.NoError(t, err)

	// Create test cases file
	testCasesContent := `cases:
  - name: "should remove field"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      keepMe: value
      removeMe: gone
    expected: |
      keepMe: value
`
	err = os.WriteFile(filepath.Join(conversionsDir, testCasesFile), []byte(testCasesContent), 0600)
	require.NoError(t, err)

	tester := NewTester()
	result, err := tester.Run(tmpDir)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, test.TestTypeConversions, result.Type)
	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Passed)
	assert.Equal(t, "should remove field", result.Results[0].Name)
}

func TestTester_Run_FailingTest(t *testing.T) {
	// Create a temp directory with failing test
	tmpDir := t.TempDir()
	conversionsDir := filepath.Join(tmpDir, "openapi", "conversions")
	err := os.MkdirAll(conversionsDir, 0755)
	require.NoError(t, err)

	// Create conversion file v2.yaml
	conversionContent := `version: 2
description:
  en: "Test conversion"
  ru: "Тестовая конверсия"
conversions:
  - del(.removeMe)
`
	err = os.WriteFile(filepath.Join(conversionsDir, "v2.yaml"), []byte(conversionContent), 0600)
	require.NoError(t, err)

	// Create test cases file with wrong expected value
	testCasesContent := `cases:
  - name: "should fail - wrong expected"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      keepMe: value
      removeMe: gone
    expected: |
      keepMe: value
      removeMe: gone
`
	err = os.WriteFile(filepath.Join(conversionsDir, testCasesFile), []byte(testCasesContent), 0600)
	require.NoError(t, err)

	tester := NewTester()
	result, err := tester.Run(tmpDir)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Passed)
	assert.Contains(t, result.Results[0].Message, "mismatch")
}

func TestConverter_ConvertTo(t *testing.T) {
	// Create a temp directory with conversion files
	tmpDir := t.TempDir()

	// Create v2 conversion
	v2Content := `version: 2
conversions:
  - del(.field1)
`
	err := os.WriteFile(filepath.Join(tmpDir, "v2.yaml"), []byte(v2Content), 0600)
	require.NoError(t, err)

	// Create v3 conversion
	v3Content := `version: 3
conversions:
  - del(.field2)
`
	err = os.WriteFile(filepath.Join(tmpDir, "v3.yaml"), []byte(v3Content), 0600)
	require.NoError(t, err)

	converter, err := newConverter(tmpDir)
	require.NoError(t, err)

	settings := map[string]any{
		"keep":   "value",
		"field1": "remove1",
		"field2": "remove2",
	}

	// Convert from v1 to v2
	_, result, err := converter.ConvertTo(1, 2, settings)
	require.NoError(t, err)

	_, hasField1 := result["field1"]
	assert.False(t, hasField1, "field1 should be removed")
	assert.Equal(t, "remove2", result["field2"], "field2 should remain")
	assert.Equal(t, "value", result["keep"], "keep should remain")

	// Convert from v1 to v3
	settings = map[string]any{
		"keep":   "value",
		"field1": "remove1",
		"field2": "remove2",
	}
	_, result, err = converter.ConvertTo(1, 3, settings)
	require.NoError(t, err)

	_, hasField1 = result["field1"]
	_, hasField2 := result["field2"]
	assert.False(t, hasField1, "field1 should be removed")
	assert.False(t, hasField2, "field2 should be removed")
	assert.Equal(t, "value", result["keep"], "keep should remain")
}

func TestReadTestCases(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), testCasesFile)

	content := `cases:
  - name: "test1"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      key: value
    expected: |
      key: value
  - name: "test2"
    currentVersion: 2
    expectedVersion: 3
    settings: |
      another: setting
    expected: |
      another: setting
`
	err := os.WriteFile(tmpFile, []byte(content), 0600)
	require.NoError(t, err)

	testCases, err := readTestCases(tmpFile)
	require.NoError(t, err)
	assert.Len(t, testCases.Cases, 2)
	assert.Equal(t, "test1", testCases.Cases[0].Name)
	assert.Equal(t, 1, testCases.Cases[0].CurrentVersion)
	assert.Equal(t, 2, testCases.Cases[0].ExpectedVersion)
}
