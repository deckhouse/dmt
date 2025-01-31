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
	ImagesDir  = "images"
)

func (r *Rules) chartModuleRule(path string, result *errors.LintRuleErrorsList) {
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
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

type Rules struct {
	cfg *config.ImageSettings
}

func New(cfg *config.ImageSettings) *Rules {
	return &Rules{
		cfg: cfg,
	}
}

func (r *Rules) ApplyImagesRules(m *module.Module, result *errors.LintRuleErrorsList) {
	r.checkImageNamesInDockerFiles(m.GetName(), m.GetPath(), result)
	r.chartModuleRule(m.GetPath(), result)

	r.lintWerfFile(m.GetWerfFile(), result)
}
