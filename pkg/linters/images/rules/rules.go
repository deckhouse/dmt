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

func chartModuleRule(name, path string) (string, *errors.LintRuleError) {
	lintError := errors.NewLintRuleError(
		ID,
		name,
		name,
		nil,
		"Module does not contain valid %q file, module will be ignored", ChartConfigFilename,
	)

	// TODO: Chart.yaml could be absent if we have module.yaml
	yamlFile, err := os.ReadFile(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		return "", lintError
	}

	var chart struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		return "", lintError
	}

	if !IsExistsOnFilesystem(path, ValuesConfigFilename) && !IsExistsOnFilesystem(path, openapiDir) {
		return "", errors.NewLintRuleError(
			ID,
			name,
			name,
			nil,
			"Module does not contain %q file or %s folder, module will be ignored",
			ValuesConfigFilename, openapiDir,
		)
	}

	return chart.Name, nil
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func ApplyImagesRules(m *module.Module) (result errors.LintRuleErrorsList) {
	result.Merge(CheckImageNamesInDockerAndWerfFiles(m.GetName(), m.GetPath()))

	name, lintError := chartModuleRule(m.GetName(), m.GetPath())
	result.Add(lintError)
	if name == "" {
		return result
	}

	return result
}

func ModuleLabel(n string) string {
	return fmt.Sprintf("module = %s", n)
}
