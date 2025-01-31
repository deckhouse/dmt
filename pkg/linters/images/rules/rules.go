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

func (r *Rules) chartModuleRule(path string) *errors.LintRuleErrorsList {
	stat, err := os.Stat(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		stat, err = os.Stat(filepath.Join(path, ModuleConfigFilename))
		if err != nil {
			r.result.Add(
				"Module does not contain valid %q or %q file",
				ChartConfigFilename, ModuleConfigFilename)
		}
	}

	yamlFile, err := os.ReadFile(filepath.Join(path, stat.Name()))
	if err != nil {
		r.result.Add(
			"Module does not contain valid %q or %q file",
			ChartConfigFilename, ModuleConfigFilename)
	}

	var chart struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		r.result.Add(
			"Module does not contain valid %q or %q file",
			ChartConfigFilename, ModuleConfigFilename)
	}

	if chart.Name == "" {
		r.result.Add(
			"Module does not contain valid %q or %q file",
			ChartConfigFilename, ModuleConfigFilename)
	}

	if !IsExistsOnFilesystem(path, openapiDir) {
		r.result.Add("Module does not contain %q folder", openapiDir)
	}

	return r.result
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

type Rules struct {
	cfg    *config.ImageSettings
	result *errors.LintRuleErrorsList
}

func New(cfg *config.ImageSettings, result *errors.LintRuleErrorsList) *Rules {
	return &Rules{
		cfg:    cfg,
		result: result,
	}
}

func (r *Rules) ApplyImagesRules(m *module.Module) *errors.LintRuleErrorsList {
	r.result.Merge(r.checkImageNamesInDockerFiles(m.GetName(), m.GetPath()))
	r.result.Merge(r.chartModuleRule(m.GetPath()))

	r.result.Merge(r.lintWerfFile(m.GetWerfFile()))

	return r.result
}
