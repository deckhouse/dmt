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

func checkModuleYaml(moduleName, modulePath string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, moduleName).WithObjectID(moduleName)
	if slices.Contains(Cfg.SkipCheckModuleYaml, moduleName) {
		return nil
	}

	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if errs.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return result.AddF(
			"Cannot stat file %q: %s",
			ModuleConfigFilename, err.Error(),
		)
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		return result.AddF(
			"Cannot read file %q: %s",
			ModuleConfigFilename, err.Error(),
		)
	}

	var yml DeckhouseModule

	err = yaml.Unmarshal(yamlFile, &yml)
	if err != nil {
		return result.AddF(
			"Cannot parse file %q: %s",
			ModuleConfigFilename, err.Error(),
		)
	}

	if yml.Name == "" {
		result.AddF("Field %q is required", "name")
	}

	stages := []string{
		"Sandbox",
		"Incubating",
		"Graduated",
		"Deprecated",
	}

	if yml.Stage != "" && !slices.Contains(stages, yml.Stage) {
		result.AddF(
			"Field %q is not one of the following values: %q",
			"stage", strings.Join(stages, ", "),
		)
	}

	if yml.Requirements != nil {
		result.Merge(yml.Requirements.validateRequirements(moduleName))
	}

	return result
}

func (m ModuleRequirements) validateRequirements(moduleName string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, moduleName).WithObjectID(moduleName)
	if m.Deckhouse != "" {
		if _, err := semver.NewConstraint(m.Deckhouse); err != nil {
			result.AddF(
				"Invalid Deckhouse version requirement: %s",
				err.Error(),
			)
		}
	}

	if m.Kubernetes != "" {
		if _, err := semver.NewConstraint(m.Kubernetes); err != nil {
			result.AddF(
				"Invalid Kubernetes version requirement: %s",
				err.Error(),
			)
		}
	}

	for parentModuleName, parentModuleVersion := range m.ParentModules {
		if _, err := semver.NewConstraint(parentModuleVersion); err != nil {
			result.AddF(
				"Invalid module %q version requirement: %s",
				parentModuleName, err.Error(),
			)
		}
	}

	return result
}
