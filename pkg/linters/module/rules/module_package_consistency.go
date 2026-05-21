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
	"strings"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const ModulePackageConsistencyRuleName = "module-package-consistency"

// NewModulePackageConsistencyRule creates a rule for cross-validating module.yaml against package.yaml.
func NewModulePackageConsistencyRule() *ModulePackageConsistencyRule {
	return &ModulePackageConsistencyRule{
		RuleMeta: pkg.RuleMeta{
			Name: ModulePackageConsistencyRuleName,
		},
	}
}

// ModulePackageConsistencyRule checks that module.yaml and package.yaml do not diverge
// when both files exist in the module directory.
type ModulePackageConsistencyRule struct {
	pkg.RuleMeta
}

// CheckModulePackageConsistency compares overlapping fields between module.yaml and package.yaml.
// Skips modules that have only one of the two files — without both there is nothing to cross-validate.
func (r *ModulePackageConsistencyRule) CheckModulePackageConsistency(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	module, err := getDeckhouseModule(modulePath, errorList.WithFilePath(ModuleConfigFilename))
	if err != nil {
		return
	}

	// load package.yaml separately so its parse errors are reported under its own file path
	packageYAML, err := getModulePackage(modulePath, errorList.WithFilePath(PackageConfigFilename))
	if err != nil {
		return
	}

	if module == nil || packageYAML == nil {
		return
	}

	compareNames(module, packageYAML, errorList)
	compareDeckhouse(module, packageYAML, errorList)
	compareKubernetes(module, packageYAML, errorList)
	compareModules(module, packageYAML, errorList)
}

// compareNames ensures module.yaml name matches package.yaml name.
func compareNames(module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Name == "" {
		return
	}

	if module.Name != packageYAML.Name {
		errorList.WithFilePath(ModuleConfigFilename).Errorf("module.yaml name %q does not match package.yaml name %q", module.Name, packageYAML.Name)
	}
}

// compareDeckhouse ensures requirements.deckhouse in module.yaml matches requirements.deckhouse.constraint in package.yaml.
func compareDeckhouse(module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Requirements == nil || module.Requirements.Deckhouse == "" {
		return
	}

	moduleConstraint := strings.TrimSpace(module.Requirements.Deckhouse)

	if packageYAML.Requirements == nil {
		errorList.WithFilePath(ModuleConfigFilename).Errorf("module.yaml requirements.deckhouse is %q but package.yaml has no requirements section", moduleConstraint)
		return
	}

	pkgConstraint := strings.TrimSpace(packageYAML.Requirements.Deckhouse.Constraint)

	if pkgConstraint == "" {
		errorList.WithFilePath(ModuleConfigFilename).Errorf("module.yaml requirements.deckhouse is %q but package.yaml requirements.deckhouse.constraint is empty", moduleConstraint)
		return
	}

	if moduleConstraint != pkgConstraint {
		errorList.WithFilePath(ModuleConfigFilename).Errorf("module.yaml requirements.deckhouse %q does not match package.yaml requirements.deckhouse.constraint %q", moduleConstraint, pkgConstraint)
	}
}

// compareKubernetes ensures requirements.kubernetes in module.yaml matches requirements.kubernetes.constraint in package.yaml.
func compareKubernetes(module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Requirements == nil || module.Requirements.Kubernetes == "" {
		return
	}

	moduleConstraint := strings.TrimSpace(module.Requirements.Kubernetes)

	if packageYAML.Requirements == nil {
		errorList.WithFilePath(ModuleConfigFilename).Errorf("module.yaml requirements.kubernetes is %q but package.yaml has no requirements section", moduleConstraint)
		return
	}

	pkgConstraint := strings.TrimSpace(packageYAML.Requirements.Kubernetes.Constraint)

	if pkgConstraint == "" {
		errorList.WithFilePath(ModuleConfigFilename).Errorf("module.yaml requirements.kubernetes is %q but package.yaml requirements.kubernetes.constraint is empty", moduleConstraint)
		return
	}

	if moduleConstraint != pkgConstraint {
		errorList.WithFilePath(ModuleConfigFilename).Errorf("module.yaml requirements.kubernetes %q does not match package.yaml requirements.kubernetes.constraint %q", moduleConstraint, pkgConstraint)
	}
}

// compareModules cross-validates module dependency lists between module.yaml and package.yaml.
// In module.yaml dependencies are a flat map (optional ones carry the "!optional" suffix),
// while package.yaml splits them into mandatory and conditional groups.
func compareModules(module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	moduleErr := errorList.WithFilePath(ModuleConfigFilename)
	pkgErr := errorList.WithFilePath(PackageConfigFilename)

	if module.Requirements == nil || len(module.Requirements.ParentModules) == 0 {
		if packageYAML.Requirements != nil && (len(packageYAML.Requirements.Modules.Mandatory) > 0 || len(packageYAML.Requirements.Modules.Conditional) > 0) {
			checkPackageModulesNotInModule(module, packageYAML, pkgErr)
		}

		return
	}

	if packageYAML.Requirements == nil {
		for name := range module.Requirements.ParentModules {
			moduleErr.Errorf("module.yaml module %q has requirement but package.yaml has no requirements section", name)
		}

		return
	}

	moduleMandatory := make(map[string]string)
	moduleConditional := make(map[string]string)

	for name, constraint := range module.Requirements.ParentModules {
		constraint = strings.TrimSpace(constraint)

		if strings.Contains(constraint, "!optional") {
			moduleConditional[name] = strings.TrimSpace(strings.ReplaceAll(constraint, "!optional", ""))
		} else {
			moduleMandatory[name] = constraint
		}
	}

	pkgMandatory := make(map[string]string)
	for _, m := range packageYAML.Requirements.Modules.Mandatory {
		pkgMandatory[m.Name] = strings.TrimSpace(m.Constraint)
	}

	pkgConditional := make(map[string]string)
	for _, m := range packageYAML.Requirements.Modules.Conditional {
		pkgConditional[m.Name] = strings.TrimSpace(m.Constraint)
	}

	// module.yaml mandatory -> package.yaml mandatory
	for name, constraint := range moduleMandatory {
		pkgConstraint, exists := pkgMandatory[name]
		if !exists {
			if _, isConditional := pkgConditional[name]; isConditional {
				moduleErr.Errorf("module.yaml module %q is mandatory but package.yaml lists it as conditional", name)
			} else {
				moduleErr.Errorf("module.yaml module %q is mandatory but not found in package.yaml requirements.modules.mandatory", name)
			}

			continue
		}

		if constraint != pkgConstraint {
			moduleErr.Errorf("module.yaml module %q constraint %q does not match package.yaml constraint %q", name, constraint, pkgConstraint)
		}
	}

	// module.yaml conditional -> package.yaml conditional
	for name, constraint := range moduleConditional {
		pkgConstraint, exists := pkgConditional[name]
		if !exists {
			if _, isMandatory := pkgMandatory[name]; isMandatory {
				moduleErr.Errorf("module.yaml module %q is optional but package.yaml lists it as mandatory", name)
			} else {
				moduleErr.Errorf("module.yaml module %q is optional but not found in package.yaml requirements.modules.conditional", name)
			}

			continue
		}

		if constraint != pkgConstraint {
			moduleErr.Errorf("module.yaml module %q constraint %q does not match package.yaml constraint %q", name, constraint, pkgConstraint)
		}
	}

	// package.yaml mandatory -> module.yaml
	for name := range pkgMandatory {
		if _, exists := moduleMandatory[name]; exists {
			continue
		}

		if _, exists := moduleConditional[name]; exists {
			continue
		}

		pkgErr.Errorf("package.yaml module %q is mandatory but not found in module.yaml requirements.modules", name)
	}

	// package.yaml conditional -> module.yaml
	for name := range pkgConditional {
		if _, exists := moduleConditional[name]; exists {
			continue
		}

		if _, exists := moduleMandatory[name]; exists {
			continue
		}

		pkgErr.Errorf("package.yaml module %q is conditional but not found in module.yaml requirements.modules", name)
	}
}

// checkPackageModulesNotInModule reports package.yaml modules that are missing from module.yaml.
func checkPackageModulesNotInModule(module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	for _, m := range packageYAML.Requirements.Modules.Mandatory {
		if module.Requirements == nil || module.Requirements.ParentModules == nil {
			errorList.Errorf("package.yaml module %q is mandatory but module.yaml has no requirements.modules", m.Name)
			continue
		}

		if _, exists := module.Requirements.ParentModules[m.Name]; !exists {
			errorList.Errorf("package.yaml module %q is mandatory but not found in module.yaml requirements.modules", m.Name)
		}
	}

	for _, m := range packageYAML.Requirements.Modules.Conditional {
		if module.Requirements == nil || module.Requirements.ParentModules == nil {
			errorList.Errorf("package.yaml module %q is conditional but module.yaml has no requirements.modules", m.Name)
			continue
		}

		if _, exists := module.Requirements.ParentModules[m.Name]; !exists {
			errorList.Errorf("package.yaml module %q is conditional but not found in module.yaml requirements.modules", m.Name)
		}
	}
}
