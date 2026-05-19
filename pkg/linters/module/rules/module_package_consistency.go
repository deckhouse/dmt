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

func NewModulePackageConsistencyRule() *ModulePackageConsistencyRule {
	return &ModulePackageConsistencyRule{
		RuleMeta: pkg.RuleMeta{
			Name: ModulePackageConsistencyRuleName,
		},
	}
}

type ModulePackageConsistencyRule struct {
	pkg.RuleMeta
}

func (r *ModulePackageConsistencyRule) CheckModulePackageConsistency(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(ModuleConfigFilename)

	module, err := getDeckhouseModule(modulePath, errorList)
	if err != nil {
		return
	}

	// package.yaml errors are reported under its own file path
	pkgErrorList := errorList.WithFilePath(PackageConfigFilename)
	pkg, err := getModulePackage(modulePath, pkgErrorList)
	if err != nil {
		return
	}

	if module == nil || pkg == nil {
		return
	}

	compareNames(module, pkg, errorList)
	compareDeckhouse(module, pkg, errorList)
	compareKubernetes(module, pkg, errorList)
	compareModules(module, pkg, errorList)
}

func compareNames(module *DeckhouseModule, pkg *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Name == "" {
		return
	}

	if module.Name != pkg.Name {
		errorList.Errorf("module.yaml name %q does not match package.yaml name %q", module.Name, pkg.Name)
	}
}

func compareDeckhouse(module *DeckhouseModule, pkg *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Requirements == nil || module.Requirements.Deckhouse == "" {
		return
	}

	if pkg.Requirements == nil || pkg.Requirements.Deckhouse.Constraint == "" {
		errorList.Errorf("module.yaml requirements.deckhouse is %q but package.yaml requirements.deckhouse.constraint is empty", module.Requirements.Deckhouse)
		return
	}

	if module.Requirements.Deckhouse != pkg.Requirements.Deckhouse.Constraint {
		errorList.Errorf("module.yaml requirements.deckhouse %q does not match package.yaml requirements.deckhouse.constraint %q", module.Requirements.Deckhouse, pkg.Requirements.Deckhouse.Constraint)
	}
}

func compareKubernetes(module *DeckhouseModule, pkg *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Requirements == nil || module.Requirements.Kubernetes == "" {
		return
	}

	if pkg.Requirements == nil || pkg.Requirements.Kubernetes.Constraint == "" {
		errorList.Errorf("module.yaml requirements.kubernetes is %q but package.yaml requirements.kubernetes.constraint is empty", module.Requirements.Kubernetes)
		return
	}

	if module.Requirements.Kubernetes != pkg.Requirements.Kubernetes.Constraint {
		errorList.Errorf("module.yaml requirements.kubernetes %q does not match package.yaml requirements.kubernetes.constraint %q", module.Requirements.Kubernetes, pkg.Requirements.Kubernetes.Constraint)
	}
}

func compareModules(module *DeckhouseModule, pkg *ModulePackage, errorList *errors.LintRuleErrorsList) {
	if module.Requirements == nil || len(module.Requirements.ParentModules) == 0 {
		// If module.yaml has no module deps but package.yaml does, that's a discrepancy
		if pkg.Requirements != nil && (len(pkg.Requirements.Modules.Mandatory) > 0 || len(pkg.Requirements.Modules.Conditional) > 0) {
			checkPackageModulesNotInModule(module, pkg, errorList)
		}
		return
	}

	if pkg.Requirements == nil {
		for name := range module.Requirements.ParentModules {
			errorList.Errorf("module.yaml module %q has requirement but package.yaml has no requirements section", name)
		}
		return
	}

	moduleMandatory := make(map[string]string)
	moduleConditional := make(map[string]string)

	for name, constraint := range module.Requirements.ParentModules {
		if strings.Contains(constraint, "!optional") {
			moduleConditional[name] = strings.TrimSpace(strings.ReplaceAll(constraint, "!optional", ""))
		} else {
			moduleMandatory[name] = constraint
		}
	}

	pkgMandatory := make(map[string]string)
	for _, m := range pkg.Requirements.Modules.Mandatory {
		pkgMandatory[m.Name] = m.Constraint
	}

	pkgConditional := make(map[string]string)
	for _, m := range pkg.Requirements.Modules.Conditional {
		pkgConditional[m.Name] = m.Constraint
	}

	// Check module.yaml mandatory → package.yaml mandatory
	for name, constraint := range moduleMandatory {
		pkgConstraint, exists := pkgMandatory[name]
		if !exists {
			if _, isConditional := pkgConditional[name]; isConditional {
				errorList.Errorf("module.yaml module %q is mandatory but package.yaml lists it as conditional", name)
			} else {
				errorList.Errorf("module.yaml module %q is mandatory but not found in package.yaml requirements.modules.mandatory", name)
			}
			continue
		}

		if constraint != pkgConstraint {
			errorList.Errorf("module.yaml module %q constraint %q does not match package.yaml constraint %q", name, constraint, pkgConstraint)
		}
	}

	// Check module.yaml conditional → package.yaml conditional
	for name, constraint := range moduleConditional {
		pkgConstraint, exists := pkgConditional[name]
		if !exists {
			if _, isMandatory := pkgMandatory[name]; isMandatory {
				errorList.Errorf("module.yaml module %q is optional but package.yaml lists it as mandatory", name)
			} else {
				errorList.Errorf("module.yaml module %q is optional but not found in package.yaml requirements.modules.conditional", name)
			}
			continue
		}

		if constraint != pkgConstraint {
			errorList.Errorf("module.yaml module %q constraint %q does not match package.yaml constraint %q", name, constraint, pkgConstraint)
		}
	}

	// Check package.yaml mandatory → module.yaml
	for name := range pkgMandatory {
		if _, exists := moduleMandatory[name]; exists {
			continue // already checked above
		}
		if _, exists := moduleConditional[name]; exists {
			continue // already reported as "optional but listed as mandatory"
		}
		errorList.Errorf("package.yaml module %q is mandatory but not found in module.yaml requirements.modules", name)
	}

	// Check package.yaml conditional → module.yaml
	for name := range pkgConditional {
		if _, exists := moduleConditional[name]; exists {
			continue // already checked above
		}
		if _, exists := moduleMandatory[name]; exists {
			continue // already reported as "mandatory but listed as conditional"
		}
		errorList.Errorf("package.yaml module %q is conditional but not found in module.yaml requirements.modules", name)
	}
}

func checkPackageModulesNotInModule(module *DeckhouseModule, pkg *ModulePackage, errorList *errors.LintRuleErrorsList) {
	for _, m := range pkg.Requirements.Modules.Mandatory {
		if module.Requirements == nil || module.Requirements.ParentModules == nil {
			errorList.Errorf("package.yaml module %q is mandatory but module.yaml has no requirements.modules", m.Name)
			continue
		}
		if _, exists := module.Requirements.ParentModules[m.Name]; !exists {
			errorList.Errorf("package.yaml module %q is mandatory but not found in module.yaml requirements.modules", m.Name)
		}
	}

	for _, m := range pkg.Requirements.Modules.Conditional {
		if module.Requirements == nil || module.Requirements.ParentModules == nil {
			errorList.Errorf("package.yaml module %q is conditional but module.yaml has no requirements.modules", m.Name)
			continue
		}
		if _, exists := module.Requirements.ParentModules[m.Name]; !exists {
			errorList.Errorf("package.yaml module %q is conditional but not found in module.yaml requirements.modules", m.Name)
		}
	}
}
