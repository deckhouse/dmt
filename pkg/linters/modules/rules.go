package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

const (
	ChartConfigFilename  = "Chart.yaml"
	ValuesConfigFilename = "values_matrix_test.yaml"

	crdsDir    = "crds"
	openapiDir = "openapi"
	hooksDir   = "hooks"
	imagesDir  = "images"
)

var toHelmignore = []string{hooksDir, openapiDir, crdsDir, imagesDir, "enabled"}

func moduleLabel(n string) string {
	return fmt.Sprintf("module = %s", n)
}

func (o *Modules) namespaceModuleRule(name, path string) (string, *errors.LintRuleError) {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return "", errors.NewLintRuleError(
			o.Name(),
			name,
			name,
			nil,
			`Module does not contain ".namespace" file, module will be ignored`,
		)
	}
	return strings.TrimRight(string(content), " \t\n"), errors.EmptyRuleError
}

func (o *Modules) chartModuleRule(name, path string) (string, *errors.LintRuleError) {
	lintError := errors.NewLintRuleError(
		o.Name(),
		name,
		name,
		nil,
		"Module does not contain valid %q file, module will be ignored", ChartConfigFilename,
	)

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

	if !isExistsOnFilesystem(path, ValuesConfigFilename) && !isExistsOnFilesystem(path, openapiDir) {
		return "", errors.NewLintRuleError(
			o.Name(),
			name,
			name,
			nil,
			"Module does not contain %q file or %s folder, module will be ignored",
			ValuesConfigFilename, openapiDir,
		)
	}

	return chart.Name, errors.EmptyRuleError
}

func (o *Modules) helmignoreModuleRule(name, path string) *errors.LintRuleError {
	var existedFiles []string
	for _, file := range toHelmignore {
		if isExistsOnFilesystem(path, file) {
			existedFiles = append(existedFiles, file)
		}
	}

	if len(existedFiles) == 0 {
		return errors.EmptyRuleError
	}

	contentBytes, err := os.ReadFile(filepath.Join(path, ".helmignore"))
	if err != nil {
		return errors.NewLintRuleError(
			o.Name(),
			name,
			name,
			nil,
			`Module does not contain ".helmignore" file`,
		)
	}

	var moduleErrors []string
	content := string(contentBytes)
	for _, existedFile := range existedFiles {
		if strings.Contains(content, existedFile) {
			continue
		}
		moduleErrors = append(moduleErrors, existedFile)
	}

	if len(moduleErrors) > 0 {
		return errors.NewLintRuleError(
			o.Name(),
			name,
			name,
			strings.Join(moduleErrors, ", "),
			`Module does not have desired entries in ".helmignore" file`,
		)
	}
	return errors.EmptyRuleError
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func (o *Modules) applyModuleRules(m *module.Module) (result errors.LintRuleErrorsList) {
	moduleName := filepath.Base(m.GetPath())

	result.Add(o.helmignoreModuleRule(moduleName, m.GetPath()))
	result.Add(o.commonTestGoForHooks(moduleName, m.GetPath()))
	result.Merge(o.checkImageNamesInDockerAndWerfFiles(moduleName, m.GetPath()))

	name, lintError := o.chartModuleRule(moduleName, m.GetPath())
	result.Add(lintError)
	if name == "" {
		return result
	}

	namespace, lintError := o.namespaceModuleRule(moduleName, m.GetPath())
	result.Add(lintError)
	if namespace == "" {
		return result
	}

	if isExistsOnFilesystem(m.GetPath(), crdsDir) {
		result.Merge(o.crdsModuleRule(moduleName, filepath.Join(m.GetPath(), crdsDir)))
	}

	result.Merge(o.ossModuleRule(moduleName, m.GetPath()))
	result.Add(o.monitoringModuleRule(moduleName, m.GetPath(), namespace))

	for _, object := range m.GetStorage() {
		result.Add(o.promtoolRuleCheck(m, object))
	}

	return result
}
