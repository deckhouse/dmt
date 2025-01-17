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
	ValuesConfigFilename = "values_matrix_test.yaml"

	CrdsDir    = "crds"
	openapiDir = "openapi"
	HooksDir   = "hooks"
	ImagesDir  = "images"
)

const (
	ID = "images"
)

var Cfg *config.ImageSettings

func chartModuleRule(name, path string) *errors.LintRuleError {
	lintError := errors.NewLintRuleError(
		ID,
		name,
		name,
		nil,
		"Module does not contain valid %q file", ChartConfigFilename,
	)

	// TODO: Chart.yaml could be absent if we have module.yaml
	yamlFile, err := os.ReadFile(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		return lintError
	}

	var chart struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		return lintError
	}

	if !IsExistsOnFilesystem(path, ValuesConfigFilename) && !IsExistsOnFilesystem(path, openapiDir) {
		return errors.NewLintRuleError(
			ID,
			name,
			name,
			nil,
			"Module does not contain %q file or %s folder",
			ValuesConfigFilename, openapiDir,
		)
	}

	return nil
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func ApplyImagesRules(m *module.Module) (result *errors.LintRuleErrorsList) {
	result = &errors.LintRuleErrorsList{}
	result.Merge(checkImageNamesInDockerFiles(m.GetName(), m.GetPath()))
	result.Merge(lintWerfFile(m.GetName(), m.GetPath()))
	result.Add(chartModuleRule(m.GetName(), m.GetPath()))

	return result
}
