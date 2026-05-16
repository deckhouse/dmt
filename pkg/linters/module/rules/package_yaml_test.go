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

func TestGetModulePackageMissingFile(t *testing.T) {
	modulePath := t.TempDir()
	errorList := errors.NewLintRuleErrorsList()

	result, err := getModulePackage(modulePath, errorList)

	require.NoError(t, err)
	assert.Nil(t, result)
	assert.False(t, errorList.ContainsErrors())
	assert.Empty(t, errorList.GetErrors())
}

func TestGetModulePackageValidFile(t *testing.T) {
	modulePath := t.TempDir()

	content := `apiVersion: v2
name: stronghold
requirements:
  kubernetes:
    constraint: ">= 1.26"
  deckhouse:
    constraint: ">= 1.77"
  modules:
    mandatory:
      - name: stronghold
        constraint: ">= 1.0.0"
    conditional:
      - name: observability
        constraint: ">= 1.0.0"
    anyOf:
      - description: "One of the following cloud providers must be installed"
        modules:
          - name: cloud-provider-gcp
            constraint: ">= 1.5.0"
          - name: cloud-provider-aws
            constraint: ">= 2.0.0"
subscribe:
  apis:
    - autoscaling.k8s.io/v1/VerticalPodAutoscaler
    - deckhouse.io/v1alpha1/ModuleRelease
  values:
    - module: stronghold
      value: .someValues.strField
    - module: cloud-provider-yandex
      value: .values.sliceField
`

	require.NoError(t, os.WriteFile(filepath.Join(modulePath, PackageConfigFilename), []byte(content), DefaultFilePerm))

	errorList := errors.NewLintRuleErrorsList()
	result, err := getModulePackage(modulePath, errorList)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, errorList.ContainsErrors())

	assert.Equal(t, "v2", result.APIVersion)
	assert.Equal(t, "stronghold", result.Name)
	require.NotNil(t, result.Requirements)
	assert.Equal(t, ">= 1.26", result.Requirements.Kubernetes.Constraint)
	assert.Equal(t, ">= 1.77", result.Requirements.Deckhouse.Constraint)

	require.Len(t, result.Requirements.Modules.Mandatory, 1)
	assert.Equal(t, "stronghold", result.Requirements.Modules.Mandatory[0].Name)
	assert.Equal(t, ">= 1.0.0", result.Requirements.Modules.Mandatory[0].Constraint)

	require.Len(t, result.Requirements.Modules.Conditional, 1)
	assert.Equal(t, "observability", result.Requirements.Modules.Conditional[0].Name)
	assert.Equal(t, ">= 1.0.0", result.Requirements.Modules.Conditional[0].Constraint)

	require.Len(t, result.Requirements.Modules.AnyOf, 1)
	assert.Equal(t, "One of the following cloud providers must be installed", result.Requirements.Modules.AnyOf[0].Description)
	require.Len(t, result.Requirements.Modules.AnyOf[0].Modules, 2)
	assert.Equal(t, "cloud-provider-gcp", result.Requirements.Modules.AnyOf[0].Modules[0].Name)
	assert.Equal(t, ">= 1.5.0", result.Requirements.Modules.AnyOf[0].Modules[0].Constraint)
	assert.Equal(t, "cloud-provider-aws", result.Requirements.Modules.AnyOf[0].Modules[1].Name)
	assert.Equal(t, ">= 2.0.0", result.Requirements.Modules.AnyOf[0].Modules[1].Constraint)

	require.NotNil(t, result.Subscribe)
	assert.Equal(t, []string{
		"autoscaling.k8s.io/v1/VerticalPodAutoscaler",
		"deckhouse.io/v1alpha1/ModuleRelease",
	}, result.Subscribe.APIs)
	require.Len(t, result.Subscribe.Values, 2)
	assert.Equal(t, "stronghold", result.Subscribe.Values[0].Module)
	assert.Equal(t, ".someValues.strField", result.Subscribe.Values[0].Value)
	assert.Equal(t, "cloud-provider-yandex", result.Subscribe.Values[1].Module)
	assert.Equal(t, ".values.sliceField", result.Subscribe.Values[1].Value)
}

func TestGetModulePackageInvalidYAML(t *testing.T) {
	modulePath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(modulePath, PackageConfigFilename), []byte(`invalid: yaml: content: [`), DefaultFilePerm))

	errorList := errors.NewLintRuleErrorsList()
	result, err := getModulePackage(modulePath, errorList)

	require.Error(t, err)
	assert.Nil(t, result)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, `Cannot parse file "package.yaml"`)
	assert.Equal(t, PackageConfigFilename, errs[0].FilePath)
}

func TestValidatePackageConstraintsValid(t *testing.T) {
	modulePackage := &ModulePackage{
		Requirements: &PackageRequirements{
			Kubernetes: PackageVersionRequirement{Constraint: ">= 1.26"},
			Deckhouse:  PackageVersionRequirement{Constraint: ">= 1.77"},
			Modules: PackageModulesRequirements{
				Mandatory: []PackageModuleRequirement{
					{Name: "stronghold", Constraint: ">= 1.0.0"},
				},
				Conditional: []PackageModuleRequirement{
					{Name: "observability", Constraint: "~1.2.0"},
				},
				AnyOf: []PackageAnyOfRequirement{
					{
						Description: "cloud provider",
						Modules: []PackageModuleRequirement{
							{Name: "cloud-provider-gcp", Constraint: ">= 1.5.0"},
							{Name: "cloud-provider-aws", Constraint: "< 2.0.0"},
						},
					},
				},
			},
		},
	}

	errorList := errors.NewLintRuleErrorsList()
	validatePackageConstraints(modulePackage, errorList)

	assert.False(t, errorList.ContainsErrors())
	assert.Empty(t, errorList.GetErrors())
}

func TestValidatePackageMetadata(t *testing.T) {
	tests := []struct {
		name           string
		modulePackage  *ModulePackage
		expectedErrors []string
	}{
		{
			name:           "nil package",
			modulePackage:  nil,
			expectedErrors: []string{},
		},
		{
			name: "valid metadata",
			modulePackage: &ModulePackage{
				APIVersion: "v2",
				Name:       "stronghold",
			},
			expectedErrors: []string{},
		},
		{
			name: "missing apiVersion",
			modulePackage: &ModulePackage{
				Name: "stronghold",
			},
			expectedErrors: []string{"package.yaml apiVersion is required"},
		},
		{
			name: "missing name",
			modulePackage: &ModulePackage{
				APIVersion: "v2",
			},
			expectedErrors: []string{"package.yaml name is required"},
		},
		{
			name:           "missing apiVersion and name",
			modulePackage:  &ModulePackage{},
			expectedErrors: []string{"package.yaml apiVersion is required", "package.yaml name is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			validatePackageMetadata(tt.modulePackage, errorList)

			errs := errorList.GetErrors()
			require.Len(t, errs, len(tt.expectedErrors))

			for idx, expectedError := range tt.expectedErrors {
				assert.Contains(t, errs[idx].Text, expectedError)
				assert.Equal(t, PackageConfigFilename, errs[idx].FilePath)
			}
		})
	}
}

func TestValidatePackageConstraintsInvalidAsIs(t *testing.T) {
	modulePackage := &ModulePackage{
		Requirements: &PackageRequirements{
			Kubernetes: PackageVersionRequirement{Constraint: "invalid-version"},
			Deckhouse:  PackageVersionRequirement{Constraint: ">= 1.77 !optional"},
			Modules: PackageModulesRequirements{
				Mandatory: []PackageModuleRequirement{
					{Name: "stronghold", Constraint: ">= 1.0.0 !optional"},
				},
				Conditional: []PackageModuleRequirement{
					{Name: "observability", Constraint: "wrong"},
				},
				AnyOf: []PackageAnyOfRequirement{
					{
						Description: "cloud provider",
						Modules: []PackageModuleRequirement{
							{Name: "cloud-provider-gcp", Constraint: ">= 1.5.0 !optional"},
						},
					},
				},
			},
		},
	}

	errorList := errors.NewLintRuleErrorsList()
	validatePackageConstraints(modulePackage, errorList)

	errs := errorList.GetErrors()
	require.Len(t, errs, 5)

	assert.Contains(t, errs[0].Text, "Invalid package.yaml requirements.kubernetes.constraint version constraint")
	assert.Contains(t, errs[1].Text, "Invalid package.yaml requirements.deckhouse.constraint version constraint")
	assert.Contains(t, errs[1].Text, `">= 1.77 !optional"`)
	assert.Contains(t, errs[2].Text, "Invalid package.yaml requirements.modules.mandatory[0].constraint version constraint")
	assert.Contains(t, errs[2].Text, `">= 1.0.0 !optional"`)
	assert.Contains(t, errs[3].Text, "Invalid package.yaml requirements.modules.conditional[0].constraint version constraint")
	assert.Contains(t, errs[4].Text, "Invalid package.yaml requirements.modules.anyOf[0].modules[0].constraint version constraint")
	assert.Contains(t, errs[4].Text, `">= 1.5.0 !optional"`)

	for _, err := range errs {
		assert.Equal(t, PackageConfigFilename, err.FilePath)
	}
}

func TestValidatePackageConstraintsSkipsEmptyAndMissingSections(t *testing.T) {
	tests := []struct {
		name          string
		modulePackage *ModulePackage
	}{
		{
			name:          "nil package",
			modulePackage: nil,
		},
		{
			name:          "nil requirements",
			modulePackage: &ModulePackage{},
		},
		{
			name: "empty constraints",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Modules: PackageModulesRequirements{
						Mandatory: []PackageModuleRequirement{{Name: "stronghold"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			validatePackageConstraints(tt.modulePackage, errorList)

			assert.False(t, errorList.ContainsErrors())
			assert.Empty(t, errorList.GetErrors())
		})
	}
}

func TestHasNewPackageRequirementsSchema(t *testing.T) {
	tests := []struct {
		name          string
		modulePackage *ModulePackage
		expected      bool
	}{
		{
			name:          "nil package",
			modulePackage: nil,
			expected:      false,
		},
		{
			name:          "nil requirements",
			modulePackage: &ModulePackage{},
			expected:      false,
		},
		{
			name: "empty requirements",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{},
			},
			expected: false,
		},
		{
			name: "only deckhouse constraint does not trigger new schema",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Deckhouse: PackageVersionRequirement{Constraint: ">= 1.77"},
				},
			},
			expected: false,
		},
		{
			name: "kubernetes constraint triggers new schema",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Kubernetes: PackageVersionRequirement{Constraint: ">= 1.26"},
				},
			},
			expected: true,
		},
		{
			name: "mandatory modules trigger new schema",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Modules: PackageModulesRequirements{
						Mandatory: []PackageModuleRequirement{{Name: "stronghold"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "conditional modules trigger new schema",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Modules: PackageModulesRequirements{
						Conditional: []PackageModuleRequirement{{Name: "observability"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "anyOf modules trigger new schema",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Modules: PackageModulesRequirements{
						AnyOf: []PackageAnyOfRequirement{{Description: "cloud provider"}},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, hasNewPackageRequirementsSchema(tt.modulePackage))
		})
	}
}

func TestValidatePackageDeckhouseRequirement(t *testing.T) {
	tests := []struct {
		name           string
		modulePackage  *ModulePackage
		expectedErrors []string
	}{
		{
			name:           "nil package does not trigger check",
			modulePackage:  nil,
			expectedErrors: []string{},
		},
		{
			name: "only deckhouse constraint does not trigger check",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Deckhouse: PackageVersionRequirement{Constraint: ">= 1.76"},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "new schema with deckhouse 1.77 passes",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Kubernetes: PackageVersionRequirement{Constraint: ">= 1.26"},
					Deckhouse:  PackageVersionRequirement{Constraint: ">= 1.77"},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "new schema with deckhouse 1.77.0 passes",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Modules: PackageModulesRequirements{
						Mandatory: []PackageModuleRequirement{{Name: "stronghold", Constraint: ">= 1.0.0"}},
					},
					Deckhouse: PackageVersionRequirement{Constraint: ">= 1.77.0"},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "new schema without deckhouse constraint fails",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Kubernetes: PackageVersionRequirement{Constraint: ">= 1.26"},
				},
			},
			expectedErrors: []string{"package.yaml requirements.deckhouse.constraint is required when new requirements schema is used and must start no lower than 1.77.0"},
		},
		{
			name: "new schema with deckhouse below 1.77 fails",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Kubernetes: PackageVersionRequirement{Constraint: ">= 1.26"},
					Deckhouse:  PackageVersionRequirement{Constraint: ">= 1.76"},
				},
			},
			expectedErrors: []string{"package.yaml requirements.deckhouse.constraint version range should start no lower than 1.77.0 (currently: 1.76.0)"},
		},
		{
			name: "new schema with deckhouse upper bound only fails",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Kubernetes: PackageVersionRequirement{Constraint: ">= 1.26"},
					Deckhouse:  PackageVersionRequirement{Constraint: "< 1.80"},
				},
			},
			expectedErrors: []string{"package.yaml requirements.deckhouse.constraint version range should start no lower than 1.77.0"},
		},
		{
			name: "new schema with invalid deckhouse constraint does not duplicate semver error",
			modulePackage: &ModulePackage{
				Requirements: &PackageRequirements{
					Kubernetes: PackageVersionRequirement{Constraint: ">= 1.26"},
					Deckhouse:  PackageVersionRequirement{Constraint: ">= 1.77 !optional"},
				},
			},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			validatePackageDeckhouseRequirement(tt.modulePackage, errorList)

			errs := errorList.GetErrors()
			require.Len(t, errs, len(tt.expectedErrors))

			for idx, expectedError := range tt.expectedErrors {
				assert.Contains(t, errs[idx].Text, expectedError)
				assert.Equal(t, PackageConfigFilename, errs[idx].FilePath)
			}
		})
	}
}

func TestPackageYAMLRule(t *testing.T) {
	tests := []struct {
		name           string
		packageContent string
		expectedErrors []string
	}{
		{
			name:           "package.yaml is missing",
			packageContent: "",
			expectedErrors: []string{},
		},
		{
			name: "valid new requirements schema",
			packageContent: `apiVersion: v2
name: stronghold
requirements:
  kubernetes:
    constraint: ">= 1.26"
  deckhouse:
    constraint: ">= 1.77"
  modules:
    mandatory:
      - name: stronghold
        constraint: ">= 1.0.0"
    conditional:
      - name: observability
        constraint: ">= 1.0.0"
    anyOf:
      - description: "cloud provider"
        modules:
          - name: cloud-provider-gcp
            constraint: ">= 1.5.0"
`,
			expectedErrors: []string{},
		},
		{
			name: "package.yaml requires apiVersion",
			packageContent: `name: stronghold
requirements:
  deckhouse:
    constraint: ">= 1.77"
`,
			expectedErrors: []string{"package.yaml apiVersion is required"},
		},
		{
			name: "package.yaml requires name",
			packageContent: `apiVersion: v2
requirements:
  deckhouse:
    constraint: ">= 1.77"
`,
			expectedErrors: []string{"package.yaml name is required"},
		},
		{
			name: "new schema requires deckhouse constraint",
			packageContent: `apiVersion: v2
name: stronghold
requirements:
  kubernetes:
    constraint: ">= 1.26"
`,
			expectedErrors: []string{"package.yaml requirements.deckhouse.constraint is required when new requirements schema is used and must start no lower than 1.77.0"},
		},
		{
			name: "new schema requires deckhouse 1.77",
			packageContent: `apiVersion: v2
name: stronghold
requirements:
  kubernetes:
    constraint: ">= 1.26"
  deckhouse:
    constraint: ">= 1.76"
`,
			expectedErrors: []string{"package.yaml requirements.deckhouse.constraint version range should start no lower than 1.77.0 (currently: 1.76.0)"},
		},
		{
			name: "constraints are parsed as is",
			packageContent: `apiVersion: v2
name: stronghold
requirements:
  kubernetes:
    constraint: ">= 1.26"
  deckhouse:
    constraint: ">= 1.77"
  modules:
    conditional:
      - name: observability
        constraint: ">= 1.0.0 !optional"
`,
			expectedErrors: []string{"Invalid package.yaml requirements.modules.conditional[0].constraint version constraint \">= 1.0.0 !optional\""},
		},
		{
			name: "invalid deckhouse constraint does not duplicate deckhouse minimum error",
			packageContent: `apiVersion: v2
name: stronghold
requirements:
  kubernetes:
    constraint: ">= 1.26"
  deckhouse:
    constraint: ">= 1.77 !optional"
`,
			expectedErrors: []string{"Invalid package.yaml requirements.deckhouse.constraint version constraint \">= 1.77 !optional\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := t.TempDir()
			if tt.packageContent != "" {
				require.NoError(t, os.WriteFile(filepath.Join(modulePath, PackageConfigFilename), []byte(tt.packageContent), DefaultFilePerm))
			}

			errorList := errors.NewLintRuleErrorsList()
			NewPackageYAMLRule().CheckPackageYAML(modulePath, errorList)
			errs := errorList.GetErrors()
			require.Len(t, errs, len(tt.expectedErrors))

			for idx, expectedError := range tt.expectedErrors {
				assert.Contains(t, errs[idx].Text, expectedError)
				assert.Equal(t, PackageConfigFilename, errs[idx].FilePath)
				assert.Equal(t, PackageYAMLRuleName, errs[idx].RuleID)
			}
		})
	}
}
