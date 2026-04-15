/*
Copyright 2026 Flant JSC

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

package tester

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgerrors "github.com/deckhouse/dmt/pkg/errors"
)

func TestTester_Name(t *testing.T) {
	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	assert.Equal(t, "conversions", tester.Name())
}

func TestTester_Desc(t *testing.T) {
	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	assert.NotEmpty(t, tester.Desc())
}

func TestTester_NoConfigValues(t *testing.T) {
	tempDir := t.TempDir()

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tempDir)

	assert.False(t, errorList.ContainsErrors())
}

func TestTester_ConfigVersionZero(t *testing.T) {
	tempDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tempDir, "openapi"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(
		filepath.Join(tempDir, "openapi", "config-values.yaml"),
		[]byte("x-config-version: 0"),
		0644,
	)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tempDir)

	assert.False(t, errorList.ContainsErrors())
}

func TestTester_ValidConversions(t *testing.T) {
	tempDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(tempDir, "openapi", "conversions"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(
		filepath.Join(tempDir, "openapi", "config-values.yaml"),
		[]byte("x-config-version: 3"),
		0644,
	)
	require.NoError(t, err)

	v2 := `version: 2
conversions:
  - del(.auth.password)
description:
  en: "v2"
  ru: "v2 ru"`
	err = os.WriteFile(filepath.Join(tempDir, "openapi", "conversions", "v2.yaml"), []byte(v2), 0644)
	require.NoError(t, err)

	v3 := `version: 3
conversions:
  - del(.auth.allowedUserGroups)
description:
  en: "v3"
  ru: "v3 ru"`
	err = os.WriteFile(filepath.Join(tempDir, "openapi", "conversions", "v3.yaml"), []byte(v3), 0644)
	require.NoError(t, err)

	testcases := `testcases:
  - name: "test v2 conversion"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
    expected: |
      auth: {}
`
	err = os.WriteFile(filepath.Join(tempDir, "openapi", "conversions", "testcases.yaml"), []byte(testcases), 0644)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tempDir)

	assert.False(t, errorList.ContainsErrors())
}

func TestTester_MissingConversionsFolder(t *testing.T) {
	tempDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(tempDir, "openapi"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(
		filepath.Join(tempDir, "openapi", "config-values.yaml"),
		[]byte("x-config-version: 2"),
		0644,
	)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tempDir)

	assert.False(t, errorList.ContainsErrors())
}

func TestTester_MissingTestcases(t *testing.T) {
	tempDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(tempDir, "openapi", "conversions"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(
		filepath.Join(tempDir, "openapi", "config-values.yaml"),
		[]byte("x-config-version: 2"),
		0644,
	)
	require.NoError(t, err)

	v2 := `version: 2
conversions:
  - del(.auth.password)
description:
  en: "v2"
  ru: "v2 ru"`
	err = os.WriteFile(filepath.Join(tempDir, "openapi", "conversions", "v2.yaml"), []byte(v2), 0644)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tempDir)

	assert.False(t, errorList.ContainsErrors())
}

func TestConversionsTester_RunsTestcases(t *testing.T) {
	tmpDir := t.TempDir()

	openapiDir := filepath.Join(tmpDir, "openapi")
	err := os.MkdirAll(openapiDir, 0755)
	require.NoError(t, err)

	configValuesYAML := `x-config-version: 2`
	err = os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte(configValuesYAML), 0644)
	require.NoError(t, err)

	convDir := filepath.Join(openapiDir, "conversions")
	err = os.MkdirAll(convDir, 0755)
	require.NoError(t, err)

	v2yaml := `version: 2
conversions:
  - del(.auth.password) | if .auth == {} then del(.auth) end
description:
  ru: "test"
  en: "test"
`
	err = os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(v2yaml), 0644)
	require.NoError(t, err)

	testcasesYAML := `testcases:
  - name: "should delete auth.password on 1 to 2"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
        allowedUserGroups:
          - group1
    expected: |
      auth:
        allowedUserGroups:
          - group1
`
	err = os.WriteFile(filepath.Join(convDir, "testcases.yaml"), []byte(testcasesYAML), 0644)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tmpDir)
	assert.False(t, errorList.ContainsErrors())
}

func TestConversionsTester_ChainConversion(t *testing.T) {
	tmpDir := t.TempDir()

	openapiDir := filepath.Join(tmpDir, "openapi")
	err := os.MkdirAll(openapiDir, 0755)
	require.NoError(t, err)

	configValuesYAML := `x-config-version: 3`
	err = os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte(configValuesYAML), 0644)
	require.NoError(t, err)

	convDir := filepath.Join(openapiDir, "conversions")
	err = os.MkdirAll(convDir, 0755)
	require.NoError(t, err)

	v2yaml := `version: 2
conversions:
  - del(.auth.password)
description:
  ru: "v2"
  en: "v2"
`
	err = os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(v2yaml), 0644)
	require.NoError(t, err)

	v3yaml := `version: 3
conversions:
  - del(.auth)
description:
  ru: "v3"
  en: "v3"
`
	err = os.WriteFile(filepath.Join(convDir, "v3.yaml"), []byte(v3yaml), 0644)
	require.NoError(t, err)

	testcasesYAML := `testcases:
  - name: "should convert from 1 to 3 (chain 1->2->3)"
    currentVersion: 1
    expectedVersion: 3
    settings: |
      auth:
        password: secret
    expected: |
      {}
`
	err = os.WriteFile(filepath.Join(convDir, "testcases.yaml"), []byte(testcasesYAML), 0644)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tmpDir)
	assert.False(t, errorList.ContainsErrors())
}

func TestConversionsTester_InvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()

	openapiDir := filepath.Join(tmpDir, "openapi")
	err := os.MkdirAll(openapiDir, 0755)
	require.NoError(t, err)

	configValuesYAML := `x-config-version: 2`
	err = os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte(configValuesYAML), 0644)
	require.NoError(t, err)

	convDir := filepath.Join(openapiDir, "conversions")
	err = os.MkdirAll(convDir, 0755)
	require.NoError(t, err)

	invalidYaml := `version: 0
conversions:
  - del(.auth.password)
description:
  ru: "invalid"
  en: "invalid"
`
	err = os.WriteFile(filepath.Join(convDir, "v0.yaml"), []byte(invalidYaml), 0644)
	require.NoError(t, err)

	testcasesYAML := `testcases:
  - name: "test"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
    expected: |
      auth: {}
`
	err = os.WriteFile(filepath.Join(convDir, "testcases.yaml"), []byte(testcasesYAML), 0644)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tmpDir)
	assert.True(t, errorList.ContainsErrors())
	assert.Contains(t, errorList.GetErrors()[0].Text, "invalid conversion version")
}

func TestConversionsTester_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	openapiDir := filepath.Join(tmpDir, "openapi")
	err := os.MkdirAll(openapiDir, 0755)
	require.NoError(t, err)

	configValuesYAML := `x-config-version: 3`
	err = os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte(configValuesYAML), 0644)
	require.NoError(t, err)

	convDir := filepath.Join(openapiDir, "conversions")
	err = os.MkdirAll(convDir, 0755)
	require.NoError(t, err)

	v2yaml := `version: 2
conversions:
  - del(.auth.password)
description:
  ru: "v2"
  en: "v2"
`
	err = os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(v2yaml), 0644)
	require.NoError(t, err)

	testcasesYAML := `testcases:
  - name: "test"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
    expected: |
      auth: {}
`
	err = os.WriteFile(filepath.Join(convDir, "testcases.yaml"), []byte(testcasesYAML), 0644)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tmpDir)
	assert.True(t, errorList.ContainsErrors())
	assert.Contains(t, errorList.GetErrors()[0].Text, "x-config-version mismatch")
}

func TestConversionsTester_ReportsTestcaseFailure(t *testing.T) {
	tmpDir := t.TempDir()

	openapiDir := filepath.Join(tmpDir, "openapi")
	err := os.MkdirAll(openapiDir, 0755)
	require.NoError(t, err)

	configValuesYAML := `x-config-version: 2`
	err = os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte(configValuesYAML), 0644)
	require.NoError(t, err)

	convDir := filepath.Join(openapiDir, "conversions")
	err = os.MkdirAll(convDir, 0755)
	require.NoError(t, err)

	v2yaml := `version: 2
conversions:
  - del(.auth.password) | if .auth == {} then del(.auth) end
description:
  ru: "test"
  en: "test"
`
	err = os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(v2yaml), 0644)
	require.NoError(t, err)

	testcasesYAML := `testcases:
  - name: "incorrect expected output"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
    expected: |
      auth:
        password: secret
`
	err = os.WriteFile(filepath.Join(convDir, "testcases.yaml"), []byte(testcasesYAML), 0644)
	require.NoError(t, err)

	errorList := pkgerrors.NewTestErrorsList()
	tester := New(errorList)
	tester.Run(tmpDir)
	assert.True(t, errorList.ContainsErrors())

	errors := errorList.GetErrors()
	assert.GreaterOrEqual(t, len(errors), 1)

	// With structured errors, check the domain fields instead of parsing formatted text
	foundMismatch := false
	for _, e := range errors {
		if e.TestName == "incorrect expected output" {
			foundMismatch = true
			assert.NotEmpty(t, e.Got, "expected Got field to be populated")
			assert.NotEmpty(t, e.Expected, "expected Expected field to be populated")
			assert.Contains(t, e.Text, "conversion mismatch")
		}
	}
	assert.True(t, foundMismatch, "expected error with TestName 'incorrect expected output'")
}
