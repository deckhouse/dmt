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

	"gopkg.in/yaml.v3"

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

	compareNames(modulePath, module, packageYAML, errorList)
	compareDeckhouse(modulePath, module, packageYAML, errorList)
	compareKubernetes(modulePath, module, packageYAML, errorList)
	compareModules(modulePath, module, packageYAML, errorList)
}

// compareNames ensures module.yaml name matches package.yaml name.
func compareNames(modulePath string, module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Name == "" {
		return
	}

	if module.Name != packageYAML.Name {
		name := packageYAML.Name
		errorList.WithFilePath(ModuleConfigFilename).
			WithFix(func() error {
				return patchModuleYAML(modulePath, func(root *yaml.Node) bool {
					return setScalar(root, "name", name)
				})
			}).
			Errorf("module.yaml name %q does not match package.yaml name %q", module.Name, packageYAML.Name)
	}
}

// compareDeckhouse ensures requirements.deckhouse in module.yaml matches requirements.deckhouse.constraint in package.yaml.
func compareDeckhouse(modulePath string, module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	compareVersionRequirement(modulePath, "deckhouse", module.Requirements, requirementDeckhouse(module), packageYAML, errorList)
}

// compareKubernetes ensures requirements.kubernetes in module.yaml matches requirements.kubernetes.constraint in package.yaml.
func compareKubernetes(modulePath string, module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	compareVersionRequirement(modulePath, "kubernetes", module.Requirements, requirementKubernetes(module), packageYAML, errorList)
}

func requirementDeckhouse(module *DeckhouseModule) string {
	if module.Requirements == nil {
		return ""
	}

	return module.Requirements.Deckhouse
}

func requirementKubernetes(module *DeckhouseModule) string {
	if module.Requirements == nil {
		return ""
	}

	return module.Requirements.Kubernetes
}

// compareVersionRequirement validates a single platform version requirement
// (deckhouse/kubernetes) and attaches an autofix that aligns module.yaml to
// package.yaml. The "value" argument is the raw module.yaml constraint.
func compareVersionRequirement(modulePath, key string, moduleReq *ModuleRequirements, value string, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if moduleReq == nil || value == "" {
		return
	}

	moduleConstraint := strings.TrimSpace(value)
	errorList = errorList.WithFilePath(ModuleConfigFilename)

	if packageYAML.Requirements == nil {
		errorList.
			WithFix(removeRequirementFix(modulePath, key)).
			Errorf("module.yaml requirements.%s is %q but package.yaml has no requirements section", key, moduleConstraint)

		return
	}

	pkgConstraint := strings.TrimSpace(packageVersionConstraint(packageYAML, key))

	if pkgConstraint == "" {
		errorList.
			WithFix(removeRequirementFix(modulePath, key)).
			Errorf("module.yaml requirements.%s is %q but package.yaml requirements.%s.constraint is empty", key, moduleConstraint, key)

		return
	}

	if moduleConstraint != pkgConstraint {
		errorList.
			WithFix(setRequirementFix(modulePath, key, pkgConstraint)).
			Errorf("module.yaml requirements.%s %q does not match package.yaml requirements.%s.constraint %q", key, moduleConstraint, key, pkgConstraint)
	}
}

func packageVersionConstraint(packageYAML *ModulePackage, key string) string {
	switch key {
	case "deckhouse":
		return packageYAML.Requirements.Deckhouse.Constraint
	case "kubernetes":
		return packageYAML.Requirements.Kubernetes.Constraint
	default:
		return ""
	}
}

func setRequirementFix(modulePath, key, value string) errors.AutofixFunc {
	return func() error {
		return patchModuleYAML(modulePath, func(root *yaml.Node) bool {
			return setRequirementScalar(root, key, value)
		})
	}
}

func removeRequirementFix(modulePath, key string) errors.AutofixFunc {
	return func() error {
		return patchModuleYAML(modulePath, func(root *yaml.Node) bool {
			return removeRequirementKey(root, key)
		})
	}
}

// compareModules cross-validates module dependency lists between module.yaml and package.yaml.
// In module.yaml dependencies are a flat map (optional ones carry the "!optional" suffix),
// while package.yaml splits them into mandatory and conditional groups.
func compareModules(modulePath string, module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Requirements == nil || len(module.Requirements.ParentModules) == 0 {
		if packageYAML.Requirements != nil && (len(packageYAML.Requirements.Modules.Mandatory) > 0 || len(packageYAML.Requirements.Modules.Conditional) > 0) {
			checkPackageModulesNotInModule(modulePath, module, packageYAML, errorList.WithFilePath(PackageConfigFilename))
		}

		return
	}

	if packageYAML.Requirements == nil {
		for name := range module.Requirements.ParentModules {
			errorList.WithFilePath(ModuleConfigFilename).
				WithFix(removeModuleFix(modulePath, name)).
				Errorf("module.yaml module %q has requirement but package.yaml has no requirements section", name)
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

	moduleErrorList := errorList.WithFilePath(ModuleConfigFilename)

	// module.yaml mandatory -> package.yaml mandatory
	for name, constraint := range moduleMandatory {
		pkgConstraint, exists := pkgMandatory[name]
		if !exists {
			if pkgCond, isConditional := pkgConditional[name]; isConditional {
				moduleErrorList.
					WithFix(setModuleFix(modulePath, name, optionalModuleValue(pkgCond))).
					Errorf("module.yaml module %q is mandatory but package.yaml lists it as conditional", name)
			} else {
				moduleErrorList.
					WithFix(removeModuleFix(modulePath, name)).
					Errorf("module.yaml module %q is mandatory but not found in package.yaml requirements.modules.mandatory", name)
			}

			continue
		}

		if constraint != pkgConstraint {
			moduleErrorList.
				WithFix(setModuleFix(modulePath, name, pkgConstraint)).
				Errorf("module.yaml module %q constraint %q does not match package.yaml constraint %q", name, constraint, pkgConstraint)
		}
	}

	// module.yaml conditional -> package.yaml conditional
	for name, constraint := range moduleConditional {
		pkgConstraint, exists := pkgConditional[name]
		if !exists {
			if pkgMand, isMandatory := pkgMandatory[name]; isMandatory {
				moduleErrorList.
					WithFix(setModuleFix(modulePath, name, pkgMand)).
					Errorf("module.yaml module %q is optional but package.yaml lists it as mandatory", name)
			} else {
				moduleErrorList.
					WithFix(removeModuleFix(modulePath, name)).
					Errorf("module.yaml module %q is optional but not found in package.yaml requirements.modules.conditional", name)
			}

			continue
		}

		if constraint != pkgConstraint {
			moduleErrorList.
				WithFix(setModuleFix(modulePath, name, optionalModuleValue(pkgConstraint))).
				Errorf("module.yaml module %q constraint %q does not match package.yaml constraint %q", name, constraint, pkgConstraint)
		}
	}

	// package.yaml mandatory -> module.yaml
	for name, constraint := range pkgMandatory {
		if _, exists := moduleMandatory[name]; exists {
			continue
		}

		if _, exists := moduleConditional[name]; exists {
			continue
		}

		errorList.WithFilePath(PackageConfigFilename).
			WithFix(setModuleFix(modulePath, name, constraint)).
			Errorf("package.yaml module %q is mandatory but not found in module.yaml requirements.modules", name)
	}

	// package.yaml conditional -> module.yaml
	for name, constraint := range pkgConditional {
		if _, exists := moduleConditional[name]; exists {
			continue
		}

		if _, exists := moduleMandatory[name]; exists {
			continue
		}

		errorList.WithFilePath(PackageConfigFilename).
			WithFix(setModuleFix(modulePath, name, optionalModuleValue(constraint))).
			Errorf("package.yaml module %q is conditional but not found in module.yaml requirements.modules", name)
	}
}

// checkPackageModulesNotInModule reports package.yaml modules that are missing from module.yaml.
func checkPackageModulesNotInModule(modulePath string, module *DeckhouseModule, packageYAML *ModulePackage, errorList *errors.LintRuleErrorsList) {
	for _, m := range packageYAML.Requirements.Modules.Mandatory {
		if module.Requirements == nil || module.Requirements.ParentModules == nil {
			errorList.
				WithFix(setModuleFix(modulePath, m.Name, strings.TrimSpace(m.Constraint))).
				Errorf("package.yaml module %q is mandatory but module.yaml has no requirements.modules", m.Name)

			continue
		}

		if _, exists := module.Requirements.ParentModules[m.Name]; !exists {
			errorList.
				WithFix(setModuleFix(modulePath, m.Name, strings.TrimSpace(m.Constraint))).
				Errorf("package.yaml module %q is mandatory but not found in module.yaml requirements.modules", m.Name)
		}
	}

	for _, m := range packageYAML.Requirements.Modules.Conditional {
		if module.Requirements == nil || module.Requirements.ParentModules == nil {
			errorList.
				WithFix(setModuleFix(modulePath, m.Name, optionalModuleValue(strings.TrimSpace(m.Constraint)))).
				Errorf("package.yaml module %q is conditional but module.yaml has no requirements.modules", m.Name)

			continue
		}

		if _, exists := module.Requirements.ParentModules[m.Name]; !exists {
			errorList.
				WithFix(setModuleFix(modulePath, m.Name, optionalModuleValue(strings.TrimSpace(m.Constraint)))).
				Errorf("package.yaml module %q is conditional but not found in module.yaml requirements.modules", m.Name)
		}
	}
}

func setModuleFix(modulePath, name, value string) errors.AutofixFunc {
	return func() error {
		return patchModuleYAML(modulePath, func(root *yaml.Node) bool {
			return setModuleEntry(root, name, value)
		})
	}
}

func removeModuleFix(modulePath, name string) errors.AutofixFunc {
	return func() error {
		return patchModuleYAML(modulePath, func(root *yaml.Node) bool {
			return removeModuleEntry(root, name)
		})
	}
}
