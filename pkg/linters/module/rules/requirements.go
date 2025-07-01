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

package rules

import (
	errs "errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	RequirementsRuleName = "requirements"
	// MinimalDeckhouseVersionForStage defines the minimum required Deckhouse version for stage usage
	MinimalDeckhouseVersionForStage = "1.68.0"
)

func NewRequirementsRule() *RequirementsRule {
	return &RequirementsRule{
		RuleMeta: pkg.RuleMeta{
			Name: RequirementsRuleName,
		},
	}
}

type RequirementsRule struct {
	pkg.RuleMeta
}

func (r *RequirementsRule) CheckRequirements(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(ModuleConfigFilename)

	module, err := getDeckhouseModule(modulePath, errorList)
	if err != nil {
		return
	}

	checkStage(module, errorList)
	checkGoHook(modulePath, module, errorList)
}

// checkStage checks if stage is used with requirements: deckhouse >= 1.68
func checkStage(module *DeckhouseModule, errorList *errors.LintRuleErrorsList) {
	if module == nil || module.Stage == "" {
		return
	}

	if module.Requirements == nil || module.Requirements.Deckhouse == "" {
		errorList.Errorf("stage should be used with requirements: deckhouse >= %s", MinimalDeckhouseVersionForStage)

		return
	}

	// Parse the constraint from requirements
	constraint, err := semver.NewConstraint(module.Requirements.Deckhouse)
	if err != nil {
		errorList.Errorf("invalid deckhouse version constraint: %s", module.Requirements.Deckhouse)

		return
	}

	// Parse the minimal required version
	minimalVersion := semver.MustParse(MinimalDeckhouseVersionForStage)

	// Check that the minimum allowed version in the range is not less than MinimalDeckhouseVersionForStage
	// For this we find the minimum lower bound among all ranges
	minAllowed := findMinimalAllowedVersion(constraint)
	if minAllowed != nil && minAllowed.LessThan(minimalVersion) {
		errorList.Errorf("requirements: for using stage, deckhouse version range should start no lower than %s (currently: %s)", MinimalDeckhouseVersionForStage, minAllowed.String())
	}
}

// checkGoHook checks if go_hook is used with requirements: deckhouse >= 1.68
func checkGoHook(modulePath string, module *DeckhouseModule, errorList *errors.LintRuleErrorsList) {
	if module == nil || module.Requirements == nil || module.Requirements.Deckhouse == "" {
		return
	}

	// Parse the constraint from requirements
	constraint, err := semver.NewConstraint(module.Requirements.Deckhouse)
	if err != nil {
		errorList.Errorf("invalid deckhouse version constraint: %s", module.Requirements.Deckhouse)

		return
	}

	// check all files in module for hooks directory
	// if hooks directory contains go files
	// if deckhouse < 1.68, then print error
	hooksDir := filepath.Join(modulePath, "hooks")
	goFiles := fsutils.GetFiles(hooksDir, true, fsutils.FilterFileByExtensions(".go"))

	if len(goFiles) == 0 {
		return
	}

	minAllowed := findMinimalAllowedVersion(constraint)
	if minAllowed != nil && minAllowed.LessThan(semver.MustParse(MinimalDeckhouseVersionForStage)) {
		errorList.Errorf("requirements: for using go_hook, deckhouse version range should start no lower than %s (currently: %s)", MinimalDeckhouseVersionForStage, minAllowed.String())

		return
	}
}

// findMinimalAllowedVersion finds the minimum allowed version among all >=, >, = in the constraint string
func findMinimalAllowedVersion(constraint *semver.Constraints) *semver.Version {
	if constraint == nil {
		return nil
	}

	pattern := regexp.MustCompile(`([><=]=?)\s*v?(\d+\.\d+\.\d+)`) // finds >= 1.2.3, > 1.2.3, = 1.2.3
	matches := pattern.FindAllStringSubmatch(constraint.String(), -1)
	var minVersion *semver.Version
	for _, m := range matches {
		op := m[1]
		verStr := m[2]
		if op == ">=" || op == ">" || op == "=" {
			v, err := semver.NewVersion(verStr)
			if err == nil {
				if minVersion == nil || v.LessThan(minVersion) {
					minVersion = v
				}
			}
		}
	}
	return minVersion
}

// getDeckhouseModule parse module.yaml file and return DeckhouseModule struct
func getDeckhouseModule(modulePath string, errorList *errors.LintRuleErrorsList) (*DeckhouseModule, error) {
	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if errs.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		errorList.Errorf("Cannot stat file %q: %s", ModuleConfigFilename, err)

		return nil, err
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		errorList.Errorf("Cannot read file %q: %s", ModuleConfigFilename, err)

		return nil, err
	}

	var yml DeckhouseModule

	err = yaml.Unmarshal(yamlFile, &yml)
	if err != nil {
		errorList.Errorf("Cannot parse file %q: %s", ModuleConfigFilename, err)

		return nil, err
	}

	return &yml, nil
}
