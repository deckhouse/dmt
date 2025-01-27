package module

import (
	errs "errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ModuleConfigFilename = "module.yaml"
)

type DeckhouseModule struct {
	Name         string              `yaml:"name"`
	Weight       uint32              `yaml:"weight,omitempty"`
	Tags         []string            `yaml:"tags"`
	Stage        string              `yaml:"stage"`
	Description  string              `yaml:"description"`
	Requirements *ModuleRequirements `yaml:"requirements,omitempty"`
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

func checkModuleYaml(moduleName, modulePath string) errors.LintRuleErrorsList {
	lintRuleErrorsList := errors.LintRuleErrorsList{}
	if slices.Contains(Cfg.SkipCheckModuleYaml, moduleName) {
		return lintRuleErrorsList
	}

	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if errs.Is(err, os.ErrNotExist) {
		return lintRuleErrorsList
	}
	if err != nil {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Cannot stat file %q: %s",
			ModuleConfigFilename, err.Error(),
		))

		return lintRuleErrorsList
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Cannot read file %q: %s",
			ModuleConfigFilename, err.Error(),
		))

		return lintRuleErrorsList
	}

	var yml DeckhouseModule

	err = yaml.Unmarshal(yamlFile, &yml)
	if err != nil {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Cannot parse file %q: %s",
			ModuleConfigFilename, err.Error(),
		))

		return lintRuleErrorsList
	}

	if yml.Name == "" {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Field %q is required",
			"name",
		))
	}

	stages := []string{
		"Sandbox",
		"Incubating",
		"Graduated",
		"Deprecated",
	}

	if yml.Stage != "" && !slices.Contains(stages, yml.Stage) {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Field %q is not one of the following values: %q",
			"stage", strings.Join(stages, ", "),
		))
	}

	if yml.Requirements != nil {
		lintRuleErrorsList.Merge(yml.Requirements.validateRequirements(moduleName))
	}

	return lintRuleErrorsList
}

func (m ModuleRequirements) validateRequirements(moduleName string) errors.LintRuleErrorsList {
	result := errors.LintRuleErrorsList{}
	if m.Deckhouse != "" {
		if _, err := semver.NewConstraint(m.Deckhouse); err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				"requirements",
				moduleName,
				nil,
				"Invalid Deckhouse version requirement: %s",
				err.Error(),
			))
		}
	}

	if m.Kubernetes != "" {
		if _, err := semver.NewConstraint(m.Kubernetes); err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				"requirements",
				moduleName,
				nil,
				"Invalid Kubernetes version requirement: %s",
				err.Error(),
			))
		}
	}

	for parentModuleName, parentModuleVersion := range m.ParentModules {
		if _, err := semver.NewConstraint(parentModuleVersion); err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				"requirements",
				moduleName,
				nil,
				"Invalid module %q version requirement: %s",
				parentModuleName, err.Error(),
			))
		}
	}

	return result
}
