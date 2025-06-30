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

	semver "github.com/blang/semver/v4"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"sigs.k8s.io/yaml"
)

const (
	RequirementsRuleName = "requirements"
)

func NewRequirementsRule(disable bool) *RequirementsRule {
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
}

// checkStage checks if stage is used with requirements: deckhouse >= 1.68
func checkStage(module *DeckhouseModule, errorList *errors.LintRuleErrorsList) {
	if module == nil || module.Stage == "" {
		return
	}

	if module.Requirements == nil || module.Requirements.Deckhouse == "" {
		errorList.Errorf("stage should be used with requirements: deckhouse >= 1.68")

		return
	}

	// deckhouse range contains string like `>= 1.68`, we should parse it as semver
	deckhouseRange, err := semver.ParseRange(module.Requirements.Deckhouse)
	if err != nil {
		errorList.Errorf("invalid deckhouse version: %s", module.Requirements.Deckhouse)

		return
	}

	if !deckhouseRange(semver.MustParse("1.68.0")) {
		errorList.Errorf("stage should be used with requirements: deckhouse >= 1.68")

		return
	}
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

	var yml *DeckhouseModule

	err = yaml.Unmarshal(yamlFile, yml)
	if err != nil {
		errorList.Errorf("Cannot parse file %q: %s", ModuleConfigFilename, err)

		return nil, err
	}

	return yml, nil
}
