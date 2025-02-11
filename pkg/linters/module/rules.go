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

func (*Module) checkModuleYaml(modulePath string, lintError *errors.Error) {
	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if errs.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		lintError.Add(
			"Cannot stat file %q: %s",
			ModuleConfigFilename, err.Error(),
		)
		return
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		lintError.Add(
			"Cannot read file %q: %s",
			ModuleConfigFilename, err.Error(),
		)
		return
	}

	var dml DeckhouseModule

	err = yaml.Unmarshal(yamlFile, &dml)
	if err != nil {
		lintError.Add(
			"Cannot parse file %q: %s",
			ModuleConfigFilename, err.Error(),
		)
		return
	}

	if dml.Name == "" {
		lintError.Add("Field %q is required", "name")
	}

	stages := []string{
		"Sandbox",
		"Incubating",
		"Graduated",
		"Deprecated",
	}

	if dml.Stage != "" && !slices.Contains(stages, dml.Stage) {
		lintError.Add(
			"Field %q is not one of the following values: %q",
			"stage", strings.Join(stages, ", "),
		)
	}

	if dml.Requirements != nil {
		dml.Requirements.validateRequirements(lintError)
	}
}

func (m ModuleRequirements) validateRequirements(lintError *errors.Error) {
	if m.Deckhouse != "" {
		if _, err := semver.NewConstraint(m.Deckhouse); err != nil {
			lintError.Add(
				"Invalid Deckhouse version requirement: %s",
				err.Error(),
			)
		}
	}

	if m.Kubernetes != "" {
		if _, err := semver.NewConstraint(m.Kubernetes); err != nil {
			lintError.Add(
				"Invalid Kubernetes version requirement: %s",
				err.Error(),
			)
		}
	}

	for parentModuleName, parentModuleVersion := range m.ParentModules {
		if _, err := semver.NewConstraint(parentModuleVersion); err != nil {
			lintError.Add(
				"Invalid module %q version requirement: %s",
				parentModuleName, err.Error(),
			)
		}
	}
}
