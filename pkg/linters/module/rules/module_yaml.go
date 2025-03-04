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
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	DefinitionFileRuleName = "definition-file"
)

func NewDefinitionFileRule(disable bool) *DefinitionFileRule {
	return &DefinitionFileRule{
		RuleMeta: pkg.RuleMeta{
			Name: DefinitionFileRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
	}
}

type DefinitionFileRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

const (
	ModuleConfigFilename = "module.yaml"
)

type DeckhouseModule struct {
	Name         string              `yaml:"name"`
	Weight       uint32              `yaml:"weight,omitempty"`
	Tags         []string            `yaml:"tags"`
	Stage        string              `yaml:"stage"`
	Descriptions ModuleDescriptions  `yaml:"descriptions,omitempty"`
	Requirements *ModuleRequirements `yaml:"requirements,omitempty"`
}

type ModuleDescriptions struct {
	English string `yaml:"en,omitempty"`
	Russian string `yaml:"ru,omitempty"`
}

type ModuleRequirements struct {
	ModulePlatformRequirements `yaml:",inline"`
	ParentModules              map[string]string `yaml:"modules,omitempty"`
}

type ModulePlatformRequirements struct {
	Deckhouse    string `yaml:"deckhouse,omitempty"`
	Kubernetes   string `yaml:"kubernetes,omitempty"`
	Bootstrapped string `yaml:"bootstrapped,omitempty"`
}

func (r *DefinitionFileRule) CheckDefinitionFile(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(ModuleConfigFilename).WithEnabled(r.Enabled())

	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if errs.Is(err, os.ErrNotExist) {
		return
	}

	if err != nil {
		errorList.Errorf("Cannot stat file %q: %s", ModuleConfigFilename, err)

		return
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		errorList.Errorf("Cannot read file %q: %s", ModuleConfigFilename, err)

		return
	}

	var yml DeckhouseModule

	err = yaml.Unmarshal(yamlFile, &yml)
	if err != nil {
		errorList.Errorf("Cannot parse file %q: %s", ModuleConfigFilename, err)

		return
	}

	if yml.Name == "" {
		errorList.Error("Field 'name' is required")
	}

	stages := []string{
		"Sandbox",
		"Incubating",
		"Graduated",
		"Deprecated",
	}

	if yml.Stage != "" && !slices.Contains(stages, yml.Stage) {
		errorList.Errorf("Field 'stage' is not one of the following values: %q", strings.Join(stages, ", "))
	}

	if yml.Requirements != nil {
		yml.Requirements.validateRequirements(errorList)
	}

	// ru description is not required
	if yml.Descriptions.English == "" {
		errorList.Warn("Module `descriptions.en` field is required")
	}
}

func (m ModuleRequirements) validateRequirements(errorList *errors.LintRuleErrorsList) {
	if m.Deckhouse != "" {
		if _, err := semver.NewConstraint(m.Deckhouse); err != nil {
			errorList.Errorf("Invalid Deckhouse version requirement: %s", err)
		}
	}

	if m.Kubernetes != "" {
		if _, err := semver.NewConstraint(m.Kubernetes); err != nil {
			errorList.Errorf("Invalid Kubernetes version requirement: %s", err)
		}
	}

	for parentModuleName, parentModuleVersion := range m.ParentModules {
		if _, err := semver.NewConstraint(parentModuleVersion); err != nil {
			errorList.Errorf("Invalid module %q version requirement: %s", parentModuleName, err)
		}
	}
}
