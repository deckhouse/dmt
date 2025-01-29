package rules

import (
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

func chartModuleRule(name, path string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name).WithObjectID(name)
	stat, err := os.Stat(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		stat, err = os.Stat(filepath.Join(path, ModuleConfigFilename))
		if err != nil {
			result.Add(
				"Module does not contain valid %q or %q file",
				ChartConfigFilename, ModuleConfigFilename)
		}
	}

	yamlFile, err := os.ReadFile(filepath.Join(path, stat.Name()))
	if err != nil {
		result.Add(
			"Module does not contain valid %q or %q file",
			ChartConfigFilename, ModuleConfigFilename)
	}

	var chart struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		result.Add(
			"Module does not contain valid %q or %q file",
			ChartConfigFilename, ModuleConfigFilename)
	}

	if chart.Name == "" {
		result.Add(
			"Module does not contain valid %q or %q file",
			ChartConfigFilename, ModuleConfigFilename)
	}

	if !IsExistsOnFilesystem(path, openapiDir) {
		result.Add("Module does not contain %q folder", openapiDir)
	}

	return result
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func ApplyImagesRules(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	result.Merge(CheckImageNamesInDockerAndWerfFiles(m.GetName(), m.GetPath()))
	result.Merge(checkImageNames(m.GetName(), m.GetPath()))
	result.Merge(chartModuleRule(m.GetName(), m.GetPath()))

	result.Merge(lintWerfFile(m.GetName(), m.GetWerfFile()))

	return result
}
