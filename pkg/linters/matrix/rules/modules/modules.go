/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

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

func namespaceModuleRule(name, path string) (string, *errors.LintRuleError) {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return "", errors.NewLintRuleError(
			"MODULE002",
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
		"MODULE002",
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
			"MODULE002",
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
			"MODULE001",
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
			"MODULE001",
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

// LintModuleStructure collects linting errors
// for helmignore, hooks, docker and werf files, namespace, and CRDs
func LintModuleStructure(modulePath string) (lintRuleErrorsList errors.LintRuleErrorsList) {
	lintRuleErrorsList = errors.LintRuleErrorsList{}
	moduleName := filepath.Base(modulePath)

	lintRuleErrorsList.Add(helmignoreModuleRule(moduleName, modulePath))
	lintRuleErrorsList.Add(commonTestGoForHooks(moduleName, modulePath))
	lintRuleErrorsList.Merge(checkImageNamesInDockerAndWerfFiles(moduleName, modulePath))

	name, lintError := chartModuleRule(moduleName, modulePath)
	lintRuleErrorsList.Add(lintError)
	if name == "" {
		return lintRuleErrorsList
	}

	namespace, lintError := namespaceModuleRule(moduleName, modulePath)
	lintRuleErrorsList.Add(lintError)
	if namespace == "" {
		return lintRuleErrorsList
	}

	if isExistsOnFilesystem(modulePath, crdsDir) {
		lintRuleErrorsList.Merge(crdsModuleRule(moduleName, filepath.Join(modulePath, crdsDir)))
	}

	lintRuleErrorsList.Merge(ossModuleRule(moduleName, modulePath))
	lintRuleErrorsList.Add(monitoringModuleRule(moduleName, modulePath, namespace))

	return lintRuleErrorsList
}
