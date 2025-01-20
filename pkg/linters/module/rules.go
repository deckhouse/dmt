package module

import (
	errs "errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ModuleConfigFilename = "module.yaml"
)

func checkModuleYaml(moduleName, modulePath string) (lintRuleErrorsList errors.LintRuleErrorsList) {
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

	var yml struct {
		Name        string   `yaml:"name"`
		Weight      uint32   `yaml:"weight,omitempty"`
		Tags        []string `yaml:"tags"`
		Stage       string   `yaml:"stage"`
		Description string   `yaml:"description"`
	}

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

	if yml.Weight != 0 && (yml.Weight < 900 || yml.Weight > 999) {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Field %q must be in range [900, 999]",
			"weight",
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

	return lintRuleErrorsList
}
