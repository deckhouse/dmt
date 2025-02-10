package module

import (
	errs "errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

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

func (l *Module) checkModuleYaml(moduleName, modulePath string) {
	if slices.Contains(l.cfg.SkipCheckModuleYaml, moduleName) {
		return
	}

	errorList := l.ErrorList.WithModule(moduleName)

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
		yml.Requirements.validateRequirements(moduleName, errorList)
	}
}

func (m ModuleRequirements) validateRequirements(moduleName string, errorList *errors.LintRuleErrorsList) {
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
