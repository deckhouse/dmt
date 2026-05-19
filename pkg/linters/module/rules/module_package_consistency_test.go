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

func runConsistencyCheck(modulePath string) *errors.LintRuleErrorsList {
	errorList := errors.NewLintRuleErrorsList()
	NewModulePackageConsistencyRule().CheckModulePackageConsistency(modulePath, errorList)

	return errorList
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if content == "" {
		return
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), DefaultFilePerm))
}

// --- No files / single file ---

func TestConsistencyNoFiles(t *testing.T) {
	modulePath := t.TempDir()
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyOnlyModuleYAML(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\n")
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyOnlyPackageYAML(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\n")
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

// --- Name comparison ---

func TestConsistencyNameMatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\n")
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyNameMismatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: different-name\napiVersion: v2\n")
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml name "stronghold" does not match package.yaml name "different-name"`)
	assert.Equal(t, ModuleConfigFilename, errs[0].FilePath)
	assert.Equal(t, ModulePackageConsistencyRuleName, errs[0].RuleID)
}

func TestConsistencyNameEmptyInModule(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "namespace: test\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\n")
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

// --- Deckhouse comparison ---

func TestConsistencyDeckhouseMatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\nrequirements:\n  deckhouse: \">= 1.77\"\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\nrequirements:\n  deckhouse:\n    constraint: \">= 1.77\"\n")
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyDeckhouseMismatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\nrequirements:\n  deckhouse: \">= 1.77\"\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\nrequirements:\n  deckhouse:\n    constraint: \">= 1.76\"\n")
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml requirements.deckhouse ">= 1.77" does not match package.yaml requirements.deckhouse.constraint ">= 1.76"`)
}

func TestConsistencyDeckhouseMissingInPackage(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\nrequirements:\n  deckhouse: \">= 1.77\"\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\nrequirements:\n  deckhouse:\n    constraint: \"\"\n")
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml requirements.deckhouse is ">= 1.77" but package.yaml requirements.deckhouse.constraint is empty`)
}

func TestConsistencyDeckhouseNotInModule(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\nrequirements:\n  deckhouse:\n    constraint: \">= 1.77\"\n")
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

// --- Kubernetes comparison ---

func TestConsistencyKubernetesMatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\nrequirements:\n  kubernetes: \">= 1.27\"\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\nrequirements:\n  kubernetes:\n    constraint: \">= 1.27\"\n")
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyKubernetesMismatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, "name: stronghold\nnamespace: test\nrequirements:\n  kubernetes: \">= 1.27\"\n")
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\nrequirements:\n  kubernetes:\n    constraint: \">= 1.26\"\n")
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml requirements.kubernetes ">= 1.27" does not match package.yaml requirements.kubernetes.constraint ">= 1.26"`)
}

// --- Module dependencies comparison ---

func TestConsistencyModulesMandatoryMatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    mandatory:
      - name: some-module
        constraint: ">= 1.0.0"
`)
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyModulesConditionalMatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0 !optional"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    conditional:
      - name: some-module
        constraint: ">= 1.0.0"
`)
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyModulesMandatoryNotFoundInPackage(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    mandatory: []
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml module "some-module" is mandatory but not found in package.yaml requirements.modules.mandatory`)
}

func TestConsistencyModulesOptionalNotFoundInPackage(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0 !optional"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    conditional: []
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml module "some-module" is optional but not found in package.yaml requirements.modules.conditional`)
}

func TestConsistencyModulesMandatoryVsConditional(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    conditional:
      - name: some-module
        constraint: ">= 1.0.0"
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml module "some-module" is mandatory but package.yaml lists it as conditional`)
}

func TestConsistencyModulesOptionalVsMandatory(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0 !optional"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    mandatory:
      - name: some-module
        constraint: ">= 1.0.0"
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml module "some-module" is optional but package.yaml lists it as mandatory`)
}

func TestConsistencyModulesConstraintMismatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    mandatory:
      - name: some-module
        constraint: ">= 2.0.0"
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml module "some-module" constraint ">= 1.0.0" does not match package.yaml constraint ">= 2.0.0"`)
}

func TestConsistencyPackageModuleMandatoryNotInModuleYAML(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    other-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    mandatory:
      - name: other-module
        constraint: ">= 1.0.0"
      - name: extra-module
        constraint: ">= 2.0.0"
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `package.yaml module "extra-module" is mandatory but not found in module.yaml requirements.modules`)
}

func TestConsistencyPackageModuleConditionalNotInModuleYAML(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    conditional:
      - name: extra-module
        constraint: ">= 1.0.0"
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `package.yaml module "extra-module" is conditional but module.yaml has no requirements.modules`)
}

func TestConsistencyModulesNoRequirementsInPackage(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    some-module: ">= 1.0.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `module.yaml module "some-module" has requirement but package.yaml has no requirements section`)
}

func TestConsistencyModulesAnyOfIgnored(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  modules:
    cloud-provider-gcp: ">= 1.5.0"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  modules:
    mandatory:
      - name: cloud-provider-gcp
        constraint: ">= 1.5.0"
    anyOf:
      - description: "cloud provider"
        modules:
          - name: cloud-provider-aws
            constraint: ">= 1.0.0"
`)
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

// --- Integration tests ---

func TestConsistencyAllMatch(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  deckhouse: ">= 1.77"
  kubernetes: ">= 1.27"
  modules:
    mandatory-mod: ">= 1.0.0"
    optional-mod: ">= 1.0.0 !optional"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: stronghold
apiVersion: v2
requirements:
  deckhouse:
    constraint: ">= 1.77"
  kubernetes:
    constraint: ">= 1.27"
  modules:
    mandatory:
      - name: mandatory-mod
        constraint: ">= 1.0.0"
    conditional:
      - name: optional-mod
        constraint: ">= 1.0.0"
`)
	errorList := runConsistencyCheck(modulePath)
	assert.False(t, errorList.ContainsErrors())
}

func TestConsistencyMultipleMismatches(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `
name: stronghold
namespace: test
requirements:
  deckhouse: ">= 1.77"
  kubernetes: ">= 1.27"
  modules:
    mandatory-mod: ">= 1.0.0"
    optional-mod: ">= 1.0.0 !optional"
`)
	writeFile(t, modulePath, PackageConfigFilename, `
name: wrong-name
apiVersion: v2
requirements:
  deckhouse:
    constraint: ">= 1.76"
  kubernetes:
    constraint: ">= 1.26"
  modules:
    mandatory: []
    conditional: []
`)
	errorList := runConsistencyCheck(modulePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 5)

	texts := make([]string, len(errs))
	for i, e := range errs {
		texts[i] = e.Text
		assert.Equal(t, ModulePackageConsistencyRuleName, e.RuleID)
	}

	assert.Contains(t, texts[0], `module.yaml name "stronghold" does not match package.yaml name "wrong-name"`)
	assert.Contains(t, texts[1], `module.yaml requirements.deckhouse ">= 1.77" does not match package.yaml requirements.deckhouse.constraint ">= 1.76"`)
	assert.Contains(t, texts[2], `module.yaml requirements.kubernetes ">= 1.27" does not match package.yaml requirements.kubernetes.constraint ">= 1.26"`)
	assert.Contains(t, texts[3], `module.yaml module "mandatory-mod" is mandatory but not found in package.yaml requirements.modules.mandatory`)
	assert.Contains(t, texts[4], `module.yaml module "optional-mod" is optional but not found in package.yaml requirements.modules.conditional`)
}

func TestConsistencyInvalidModuleYAMLIgnoresConsistency(t *testing.T) {
	modulePath := t.TempDir()
	writeFile(t, modulePath, ModuleConfigFilename, `invalid: yaml: [[[`)
	writeFile(t, modulePath, PackageConfigFilename, "name: stronghold\napiVersion: v2\n")
	errorList := runConsistencyCheck(modulePath)
	// The parse error is reported but consistency checks are skipped
	errs := errorList.GetErrors()
	// The module.yaml parse error goes through errorList (getDeckhouseModule returns err)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `Cannot parse file "module.yaml"`)
}
