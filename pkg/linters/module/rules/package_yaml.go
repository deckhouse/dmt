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
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PackageYAMLRuleName                           = "package-yaml"
	PackageConfigFilename                         = "package.yaml"
	MinimalDeckhouseVersionForPackageRequirements = "1.77.0"
)

// NewPackageYAMLRule creates a rule for validating package.yaml.
func NewPackageYAMLRule() *PackageYAMLRule {
	return &PackageYAMLRule{
		RuleMeta: pkg.RuleMeta{
			Name: PackageYAMLRuleName,
		},
	}
}

// PackageYAMLRule validates the module package.yaml file.
type PackageYAMLRule struct {
	pkg.RuleMeta
}

// ModulePackage describes package.yaml fields used by module lint rules.
type ModulePackage struct {
	APIVersion   string               `json:"apiVersion,omitempty"`
	Name         string               `json:"name,omitempty"`
	Requirements *PackageRequirements `json:"requirements,omitempty"`
	Subscribe    *PackageSubscribe    `json:"subscribe,omitempty"`
}

// PackageRequirements describes package.yaml requirements.
type PackageRequirements struct {
	Kubernetes PackageVersionRequirement  `json:"kubernetes,omitempty"`
	Deckhouse  PackageVersionRequirement  `json:"deckhouse,omitempty"`
	Modules    PackageModulesRequirements `json:"modules,omitempty"`
}

// PackageVersionRequirement describes a version constraint requirement.
type PackageVersionRequirement struct {
	Constraint string `json:"constraint,omitempty"`
}

// PackageModulesRequirements describes package.yaml module dependency groups.
type PackageModulesRequirements struct {
	Mandatory   []PackageModuleRequirement `json:"mandatory,omitempty"`
	Conditional []PackageModuleRequirement `json:"conditional,omitempty"`
	AnyOf       []PackageAnyOfRequirement  `json:"anyOf,omitempty"`
}

// PackageModuleRequirement describes a package.yaml module dependency.
type PackageModuleRequirement struct {
	Name       string `json:"name,omitempty"`
	Constraint string `json:"constraint,omitempty"`
}

// PackageAnyOfRequirement describes an anyOf module dependency group.
type PackageAnyOfRequirement struct {
	Description string                     `json:"description,omitempty"`
	Modules     []PackageModuleRequirement `json:"modules,omitempty"`
}

// PackageSubscribe describes package.yaml subscribe settings.
type PackageSubscribe struct {
	APIs   []string                `json:"apis,omitempty"`
	Values []PackageSubscribeValue `json:"values,omitempty"`
}

// PackageSubscribeValue describes a subscribed module value.
type PackageSubscribeValue struct {
	Module string `json:"module,omitempty"`
	Value  string `json:"value,omitempty"`
}

// getModulePackage parses package.yaml and returns the subset of fields used by module rules.
func getModulePackage(modulePath string, errorList *errors.LintRuleErrorsList) (*ModulePackage, error) {
	errorList = errorList.WithFilePath(PackageConfigFilename)
	packageFilePath := filepath.Join(modulePath, PackageConfigFilename)

	_, err := os.Stat(packageFilePath)

	if stderrors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		errorList.Errorf("Cannot stat file %q: %s", PackageConfigFilename, err)

		return nil, err
	}

	yamlFile, err := os.ReadFile(packageFilePath)
	if err != nil {
		errorList.Errorf("Cannot read file %q: %s", PackageConfigFilename, err)

		return nil, err
	}

	var yml ModulePackage

	err = yaml.Unmarshal(yamlFile, &yml)
	if err != nil {
		errorList.Errorf("Cannot parse file %q: %s", PackageConfigFilename, err)

		return nil, err
	}

	return &yml, nil
}

// CheckPackageYAML validates package.yaml in the module root.
func (r *PackageYAMLRule) CheckPackageYAML(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	modulePackage, err := getModulePackage(modulePath, errorList)
	if err != nil {
		return
	}

	checkModulePackageRequirements(modulePackage, errorList)
}

// checkModulePackageRequirements runs all package.yaml checks.
func checkModulePackageRequirements(modulePackage *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if modulePackage == nil {
		return
	}

	validatePackageMetadata(modulePackage, errorList)
	validatePackageConstraints(modulePackage, errorList)
	validatePackageDeckhouseRequirement(modulePackage, errorList)
}

// validatePackageMetadata validates required package.yaml metadata fields.
func validatePackageMetadata(modulePackage *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if modulePackage == nil {
		return
	}

	errorList = errorList.WithFilePath(PackageConfigFilename)
	if modulePackage.APIVersion == "" {
		errorList.Error("package.yaml apiVersion is required")
	}

	if modulePackage.Name == "" {
		errorList.Error("package.yaml name is required")
	}
}

// validatePackageConstraints validates all package.yaml constraints as-is.
func validatePackageConstraints(modulePackage *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if modulePackage == nil || modulePackage.Requirements == nil {
		return
	}

	errorList = errorList.WithFilePath(PackageConfigFilename)
	requirements := modulePackage.Requirements

	validatePackageConstraint("requirements.kubernetes.constraint", requirements.Kubernetes.Constraint, errorList)
	validatePackageConstraint("requirements.deckhouse.constraint", requirements.Deckhouse.Constraint, errorList)

	for idx, module := range requirements.Modules.Mandatory {
		validatePackageConstraint(fmt.Sprintf("requirements.modules.mandatory[%d].constraint", idx), module.Constraint, errorList)
	}

	for idx, module := range requirements.Modules.Conditional {
		validatePackageConstraint(fmt.Sprintf("requirements.modules.conditional[%d].constraint", idx), module.Constraint, errorList)
	}

	for anyOfIdx, anyOf := range requirements.Modules.AnyOf {
		for moduleIdx, module := range anyOf.Modules {
			validatePackageConstraint(fmt.Sprintf("requirements.modules.anyOf[%d].modules[%d].constraint", anyOfIdx, moduleIdx), module.Constraint, errorList)
		}
	}
}

// validatePackageConstraint validates a single package.yaml version constraint.
func validatePackageConstraint(fieldPath, constraint string, errorList *errors.LintRuleErrorsList) {
	if constraint == "" {
		return
	}

	if _, err := semver.NewConstraint(constraint); err != nil {
		errorList.Errorf("Invalid package.yaml %s version constraint %q: %s", fieldPath, constraint, err)
	}
}

// hasNewPackageRequirementsSchema checks if package.yaml uses the new requirements schema.
func hasNewPackageRequirementsSchema(modulePackage *ModulePackage) bool {
	if modulePackage == nil || modulePackage.Requirements == nil {
		return false
	}

	requirements := modulePackage.Requirements

	return requirements.Kubernetes.Constraint != "" ||
		len(requirements.Modules.Mandatory) > 0 ||
		len(requirements.Modules.Conditional) > 0 ||
		len(requirements.Modules.AnyOf) > 0
}

// validatePackageDeckhouseRequirement validates the Deckhouse requirement for the new requirements schema.
func validatePackageDeckhouseRequirement(modulePackage *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if !hasNewPackageRequirementsSchema(modulePackage) {
		return
	}

	errorList = errorList.WithFilePath(PackageConfigFilename)

	deckhouseConstraint := modulePackage.Requirements.Deckhouse.Constraint

	if deckhouseConstraint == "" {
		errorList.Errorf("package.yaml requirements.deckhouse.constraint is required when new requirements schema is used and must start no lower than %s", MinimalDeckhouseVersionForPackageRequirements)
		return
	}

	constraint, err := semver.NewConstraint(deckhouseConstraint)
	if err != nil {
		return
	}

	minAllowed := findMinimalAllowedVersion(constraint)

	minimalVersion, err := semver.NewVersion(MinimalDeckhouseVersionForPackageRequirements)
	if err != nil {
		errorList.Errorf("invalid package.yaml minimum Deckhouse version format %s: %s", MinimalDeckhouseVersionForPackageRequirements, err)
		return
	}

	if minAllowed == nil || minAllowed.LessThan(minimalVersion) {
		if minAllowed == nil {
			errorList.Errorf("package.yaml requirements.deckhouse.constraint version range should start no lower than %s", MinimalDeckhouseVersionForPackageRequirements)
			return
		}

		errorList.Errorf("package.yaml requirements.deckhouse.constraint version range should start no lower than %s (currently: %s)", MinimalDeckhouseVersionForPackageRequirements, minAllowed.String())
	}
}
