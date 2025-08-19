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

	"sigs.k8s.io/yaml"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	DefinitionFileRuleName = "definition-file"
)

// Valid edition values
var ValidEditions = []string{
	"ce",
	"fe",
	"ee",
	"se",
	"se-plus",
	"be",
	"_default",
}

// Valid bundle values
var ValidBundles = []string{
	"Minimal",
	"Managed",
	"Default",
}

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
	Name          string               `json:"name"`
	Critical      bool                 `json:"critical,omitempty"`
	Namespace     string               `json:"namespace"`
	Weight        uint32               `json:"weight,omitempty"`
	Tags          []string             `json:"tags"`
	Subsystems    []string             `json:"subsystems,omitempty"`
	Stage         string               `json:"stage"`
	Description   string               `json:"description,omitempty"`
	Descriptions  ModuleDescriptions   `json:"descriptions,omitempty"`
	Requirements  *ModuleRequirements  `json:"requirements,omitempty"`
	Accessibility *ModuleAccessibility `json:"accessibility,omitempty"`
}

type ModuleDescriptions struct {
	English string `json:"en,omitempty"`
	Russian string `json:"ru,omitempty"`
}

type ModuleRequirements struct {
	ModulePlatformRequirements `json:",inline"`
	ParentModules              map[string]string `json:"modules,omitempty"`
}

type ModulePlatformRequirements struct {
	Deckhouse    string `json:"deckhouse,omitempty"`
	Kubernetes   string `json:"kubernetes,omitempty"`
	Bootstrapped bool   `json:"bootstrapped,omitempty"`
}

type ModuleAccessibility struct {
	Editions map[string]ModuleEdition `json:"editions"`
}

type ModuleEdition struct {
	Available       bool     `json:"available"`
	EnabledInBundle []string `json:"enabledInBundle"`
}

func (r *DefinitionFileRule) CheckDefinitionFile(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(ModuleConfigFilename)

	if !r.Enabled() {
		// TODO: add metrics
		return
	}

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

	const maxNameLength = 64
	if len(yml.Name) > maxNameLength {
		errorList.Error("Field 'name' must not exceed 64 characters")
	}

	stages := []string{
		"Experimental",
		"Preview",
		"General Availability",
		"Deprecated",
	}

	if yml.Stage == "" {
		errorList.Error("Field 'stage' is required")
	}

	if yml.Stage != "" && !slices.Contains(stages, yml.Stage) {
		errorList.Errorf("Field 'stage' is not one of the following values: %q", strings.Join(stages, ", "))
	}

	if yml.Requirements != nil {
		yml.Requirements.validateRequirements(errorList)
	}

	if yml.Accessibility != nil {
		yml.Accessibility.validateAccessibility(errorList)
	}

	// ru description is not required
	if yml.Descriptions.English == "" {
		errorList.Warn("Module `descriptions.en` field is required")
	}

	if yml.Critical && yml.Weight == 0 {
		errorList.Error("Field 'weight' must not be zero for critical modules")
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

func (a *ModuleAccessibility) validateAccessibility(errorList *errors.LintRuleErrorsList) {
	if len(a.Editions) == 0 {
		errorList.Error("Field 'accessibility.editions' is required when 'accessibility' is specified")
		return
	}

	for editionName, edition := range a.Editions {
		// Validate edition name
		if !slices.Contains(ValidEditions, editionName) {
			errorList.Errorf("Invalid edition name %q. Must be one of: %s", editionName, strings.Join(ValidEditions, ", "))
		}

		// Validate enabledInBundle values
		if len(edition.EnabledInBundle) == 0 {
			errorList.Errorf("Field 'enabledInBundle' is required for edition %q", editionName)
		} else {
			for _, bundle := range edition.EnabledInBundle {
				if !slices.Contains(ValidBundles, bundle) {
					errorList.Errorf("Invalid bundle %q for edition %q. Must be one of: %s", bundle, editionName, strings.Join(ValidBundles, ", "))
				}
			}
		}
	}
}
