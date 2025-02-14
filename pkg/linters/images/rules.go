package images

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/internal/module"
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
	chartRuleName = "chart"
)

func chartModuleRule(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(chartRuleName)
	errModuleNotContainValidFiles := fmt.Sprintf("Module does not contain valid %q or %q file", ChartConfigFilename, ModuleConfigFilename)

	stat, err := os.Stat(filepath.Join(modulePath, ChartConfigFilename))
	if err != nil {
		stat, err = os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
		if err != nil {
			errorList.Error(errModuleNotContainValidFiles)
		}
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, stat.Name()))
	if err != nil {
		errorList.Error(errModuleNotContainValidFiles)
	}

	var chart struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		errorList.Error(errModuleNotContainValidFiles)
	}

	if chart.Name == "" {
		errorList.Error(errModuleNotContainValidFiles)
	}

	if !IsExistsOnFilesystem(modulePath, openapiDir) {
		errorList.Errorf("Module does not contain %q folder", openapiDir)
	}
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func (l *Images) ApplyImagesRules(m *module.Module, result *errors.LintRuleErrorsList) *errors.LintRuleErrorsList {
	l.checkImageNamesInDockerFiles(m.GetName(), m.GetPath(), result)

	chartModuleRule(m.GetPath(), result)

	lintWerfFile(m.GetWerfFile(), result)

	return result
}
