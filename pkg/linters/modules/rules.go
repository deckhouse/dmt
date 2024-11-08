package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/modules/rules"
)

const (
	ChartConfigFilename  = "Chart.yaml"
	ValuesConfigFilename = "values_matrix_test.yaml"

	CrdsDir    = "crds"
	openapiDir = "openapi"
	HooksDir   = "hooks"
	ImagesDir  = "images"
)

var toHelmignore = []string{HooksDir, openapiDir, CrdsDir, ImagesDir, "enabled"}

func ModuleLabel(n string) string {
	return fmt.Sprintf("module = %s", n)
}

func namespaceModuleRule(name, path string) (string, *errors.LintRuleError) {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return "", errors.NewLintRuleError(
			ID,
			name,
			name,
			nil,
			`Module does not contain ".namespace" file, module will be ignored`,
		)
	}
	return strings.TrimRight(string(content), " \t\n"), errors.EmptyRuleError
}

func chartModuleRule(name, path string) (string, *errors.LintRuleError) {
	lintError := errors.NewLintRuleError(
		ID,
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

	return chart.Name, errors.EmptyRuleError
}

func helmignoreModuleRule(name, path string) *errors.LintRuleError {
	var existedFiles []string
	for _, file := range toHelmignore {
		if IsExistsOnFilesystem(path, file) {
			existedFiles = append(existedFiles, file)
		}
	}

	if len(existedFiles) == 0 {
		return errors.EmptyRuleError
	}

	contentBytes, err := os.ReadFile(filepath.Join(path, ".helmignore"))
	if err != nil {
		return errors.NewLintRuleError(
			ID,
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
			ID,
			name,
			name,
			strings.Join(moduleErrors, ", "),
			`Module does not have desired entries in ".helmignore" file`,
		)
	}
	return errors.EmptyRuleError
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func applyModuleRules(m *module.Module) (result errors.LintRuleErrorsList) {
	moduleName := filepath.Base(m.GetPath())

	result.Add(helmignoreModuleRule(moduleName, m.GetPath()))
	result.Add(rules.CommonTestGoForHooks(moduleName, m.GetPath()))
	result.Merge(rules.CheckImageNamesInDockerAndWerfFiles(moduleName, m.GetPath()))

	name, lintError := chartModuleRule(moduleName, m.GetPath())
	result.Add(lintError)
	if name == "" {
		return result
	}

	namespace, lintError := namespaceModuleRule(moduleName, m.GetPath())
	result.Add(lintError)
	if namespace == "" {
		return result
	}

	if IsExistsOnFilesystem(m.GetPath(), CrdsDir) {
		result.Merge(rules.CrdsModuleRule(moduleName, filepath.Join(m.GetPath(), CrdsDir)))
	}

	result.Merge(rules.OssModuleRule(moduleName, m.GetPath()))
	result.Add(rules.MonitoringModuleRule(moduleName, m.GetPath(), namespace))

	for _, object := range m.GetStorage() {
		result.Add(rules.PromtoolRuleCheck(m, object))
	}

	return result
}
