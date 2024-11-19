package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	ID = "helm"
)

var Cfg *config.HelmSettings

var toHelmignore = []string{HooksDir, openapiDir, CrdsDir, ImagesDir, "enabled"}

func namespaceModuleRule(name, path string) (string, *errors.LintRuleError) {
	if slices.Contains(Cfg.SkipNamespaceCheck, name) {
		return "", nil
	}
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
	return strings.TrimRight(string(content), " \t\n"), nil
}

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

func helmignoreModuleRule(name, path string) *errors.LintRuleError {
	if slices.Contains(Cfg.SkipHelmIgnoreCheck, name) {
		return nil
	}

	var existedFiles []string
	for _, file := range toHelmignore {
		if IsExistsOnFilesystem(path, file) {
			existedFiles = append(existedFiles, file)
		}
	}

	if len(existedFiles) == 0 {
		return nil
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
	return nil
}

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func ApplyHelmRules(m *module.Module) (result errors.LintRuleErrorsList) {
	result.Add(helmignoreModuleRule(m.GetName(), m.GetPath()))
	result.Merge(CheckImageNamesInDockerAndWerfFiles(m.GetName(), m.GetPath()))

	name, lintError := chartModuleRule(m.GetName(), m.GetPath())
	result.Add(lintError)
	if name == "" {
		return result
	}

	namespace, lintError := namespaceModuleRule(m.GetName(), m.GetPath())
	result.Add(lintError)
	if namespace == "" {
		return result
	}

	return result
}

func ModuleLabel(n string) string {
	return fmt.Sprintf("module = %s", n)
}
