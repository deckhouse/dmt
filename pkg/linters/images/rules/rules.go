package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ChartConfigFilename  = "Chart.yaml"
	ModuleConfigFilename = "module.yaml"

	CrdsDir    = "crds"
	openapiDir = "openapi"
	HooksDir   = "hooks"
	ImagesDir  = "images"
)

const (
	ID = "images"
)

var Cfg *config.ImageSettings

func chartModuleRule(name, path string) (lintRuleErrorsList errors.LintRuleErrorsList) {
	lintError := errors.NewLintRuleError(
		ID,
		name,
		name,
		nil,
		"Module does not contain valid %q or %q file",
		ChartConfigFilename, ModuleConfigFilename,
	)

	stat, err := os.Stat(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		stat, err = os.Stat(filepath.Join(path, ModuleConfigFilename))
		if err != nil {
			lintRuleErrorsList.Add(lintError)
		}
	}

	yamlFile, err := os.ReadFile(filepath.Join(path, stat.Name()))
	if err != nil {
		lintRuleErrorsList.Add(lintError)
	}

	var chart struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		lintRuleErrorsList.Add(lintError)
	}

	if chart.Name == "" {
		lintRuleErrorsList.Add(lintError)
	}

	if !IsExistsOnFilesystem(path, openapiDir) {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			ID,
			name,
			name,
			nil,
			"Module does not contain %s folder",
			openapiDir,
		))
	}

	return lintRuleErrorsList
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func ApplyImagesRules(m *module.Module) (result errors.LintRuleErrorsList) {
	result.Merge(CheckImageNamesInDockerAndWerfFiles(m.GetName(), m.GetPath()))
	result.Merge(chartModuleRule(m.GetName(), m.GetPath()))

	return result
}

func ModuleLabel(n string) string {
	return fmt.Sprintf("module = %s", n)
}
