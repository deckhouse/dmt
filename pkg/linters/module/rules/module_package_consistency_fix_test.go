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

package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

// checkAndFix mirrors the production flow: collect findings, then apply the
// fixes that were attached to them in a single pass.
func checkAndFix(t *testing.T, modulePath string) errors.FixResult {
	t.Helper()

	errorList := errors.NewLintRuleErrorsList()
	NewModulePackageConsistencyRule().CheckModulePackageConsistency(modulePath, errorList)

	result := errorList.ApplyFixes()
	require.Empty(t, result.Failed, "autofix should not fail")

	return result
}

func readModuleYAML(t *testing.T, modulePath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	require.NoError(t, err)

	return string(content)
}

// TestFixResolvesAllConsistencyFindings reproduces the documented scenario: a
// fully diverged module.yaml is fixed from package.yaml and afterwards the
// consistency check reports nothing.
func TestFixResolvesAllConsistencyFindings(t *testing.T) {
	modulePath := t.TempDir()

	writeFile(t, modulePath, ModuleConfigFilename, `name: wrong-name
namespace: test
stage: General Availability
requirements:
  deckhouse: ">=0.0.0"
  kubernetes: ">=1.19.0"
  modules:
    prompp: "!optional >=0.16.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `apiVersion: v2
name: correct-name
requirements:
  deckhouse:
    constraint: ">=0.1.0"
  kubernetes:
    constraint: ">=1.20.0"
  modules:
    mandatory:
      - name: stronghold
        constraint: ">=0.1.0"
    conditional:
      - name: prompp
        constraint: ">=0.1.0"
    anyOf:
      - description: "cloud provider"
        modules:
          - name: cloud-provider-aws
            constraint: ">=1.0.0"
`)

	// before fix: there are findings
	require.True(t, runConsistencyCheck(modulePath).ContainsErrors())

	result := checkAndFix(t, modulePath)
	assert.Positive(t, result.Applied)

	// after fix: no findings remain
	assert.False(t, runConsistencyCheck(modulePath).ContainsErrors())

	fixed := readModuleYAML(t, modulePath)

	// source-of-truth fields are aligned
	assert.Contains(t, fixed, "name: correct-name")
	assert.Contains(t, fixed, "deckhouse:")
	assert.Contains(t, fixed, ">=0.1.0")
	assert.Contains(t, fixed, ">=1.20.0")
	assert.Contains(t, fixed, "stronghold:")
	assert.Contains(t, fixed, "prompp:")
	assert.Contains(t, fixed, "!optional")

	// unrelated fields are preserved
	assert.Contains(t, fixed, "namespace: test")
	assert.Contains(t, fixed, "stage: General Availability")

	// anyOf modules are not pulled into module.yaml
	assert.NotContains(t, fixed, "cloud-provider-aws")
}

func TestFixCreatesRequirementsWhenMissing(t *testing.T) {
	modulePath := t.TempDir()

	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\n")
	writeFile(t, modulePath, PackageConfigFilename, `apiVersion: v2
name: stronghold
requirements:
  modules:
    conditional:
      - name: extra-module
        constraint: ">= 1.0.0"
`)

	checkAndFix(t, modulePath)

	assert.False(t, runConsistencyCheck(modulePath).ContainsErrors())

	fixed := readModuleYAML(t, modulePath)
	assert.Contains(t, fixed, "extra-module:")
	assert.Contains(t, fixed, "!optional")
}

func TestFixConvertsMandatoryToConditional(t *testing.T) {
	modulePath := t.TempDir()

	writeFile(t, modulePath, ModuleConfigFilename, `name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `apiVersion: v2
name: stronghold
requirements:
  modules:
    conditional:
      - name: some-module
        constraint: ">= 1.0.0"
`)

	checkAndFix(t, modulePath)

	assert.False(t, runConsistencyCheck(modulePath).ContainsErrors())
	assert.Contains(t, readModuleYAML(t, modulePath), "!optional")
}

func TestFixRemovesModuleNotInPackage(t *testing.T) {
	modulePath := t.TempDir()

	writeFile(t, modulePath, ModuleConfigFilename, `name: stronghold
namespace: test
requirements:
  modules:
    ghost-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `apiVersion: v2
name: stronghold
requirements:
  modules:
    mandatory: []
`)

	checkAndFix(t, modulePath)

	assert.False(t, runConsistencyCheck(modulePath).ContainsErrors())
	assert.NotContains(t, readModuleYAML(t, modulePath), "ghost-module")
}

func TestFixNoopWhenAlreadyConsistent(t *testing.T) {
	modulePath := t.TempDir()

	moduleContent := `name: stronghold
namespace: test
requirements:
  deckhouse: ">= 1.77"
`
	writeFile(t, modulePath, ModuleConfigFilename, moduleContent)
	writeFile(t, modulePath, PackageConfigFilename, `apiVersion: v2
name: stronghold
requirements:
  deckhouse:
    constraint: ">= 1.77"
`)

	result := checkAndFix(t, modulePath)
	assert.Zero(t, result.Applied)

	// already consistent module.yaml is left byte-for-byte unchanged
	assert.Equal(t, moduleContent, readModuleYAML(t, modulePath))
}
