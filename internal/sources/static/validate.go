/*
Copyright 2025 Flant JSC

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

package static

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/dmt/internal/modules"
	dmtErrors "github.com/deckhouse/dmt/pkg/errors"
)

// validateModule is the source tree's pre-flight check. It reports through the
// `module`/`definition-file` rule rather than under its own name, because what it
// checks is what that rule checks — the remote scopes reach the same ground through
// the bundle-layout and release-layout rules instead.
func validateModule(path string, errorList *dmtErrors.LintRuleErrorsList) error {
	var errs error

	errorList = errorList.WithLinterID("module").WithRule("definition-file").WithFilePath(path)
	// validate module.yaml and Chart.yaml
	chartYamlFile, err := modules.ParseChartFile(path)
	if err != nil {
		err = fmt.Errorf("failed to parse Chart.yaml: %w", err)
		errs = errors.Join(errs, err)
		errorList.Error(err.Error())
	}

	moduleYamlFile, err := modules.ParseModuleConfigFile(path)
	if err != nil {
		err = fmt.Errorf("failed to parse module.yaml: %w", err)
		errs = errors.Join(errs, err)
		errorList.Error(err.Error())
	}

	if chartYamlFile != nil {
		if chartYamlFile.Name == "" {
			err := errors.New("property `name` in Chart.yaml is empty")
			errs = errors.Join(errs, err)
			errorList.Error(err.Error())
		}

		if chartYamlFile.Version == "" {
			err := errors.New("property `version` in Chart.yaml is empty")
			errs = errors.Join(errs, err)
			errorList.Error(err.Error())
		}
	}

	if moduleYamlFile != nil {
		if moduleYamlFile.Name == "" {
			errorList.Warn("module.yaml `name` is empty")
		}

		if moduleYamlFile.Namespace == "" {
			errorList.Warn("module.yaml `namespace` is empty")
		}
	}

	if moduleYamlFile != nil && chartYamlFile != nil &&
		moduleYamlFile.Name != "" && chartYamlFile.Name != "" &&
		chartYamlFile.Name != moduleYamlFile.Name {
		err := fmt.Errorf("module.yaml name (%s) does not match Chart.yaml name (%s)", moduleYamlFile.Name, chartYamlFile.Name)
		errs = errors.Join(errs, err)
		errorList.Errorf("%s", err.Error())
	}

	moduleName := modules.GetModuleName(moduleYamlFile, chartYamlFile)
	if moduleName == "" && chartYamlFile == nil {
		err := fmt.Errorf("module `name` property is empty")
		errs = errors.Join(errs, err)
		errorList.Errorf("%s", err.Error())
	}

	if moduleYamlFile == nil && chartYamlFile != nil && getNamespace(path) == "" {
		err := fmt.Errorf("file Chart.yaml is present, but .namespace file is missing")
		errs = errors.Join(errs, err)
		errorList.Errorf("%s", err.Error())
	}

	if err := validateOpenAPIDir(path); err != nil {
		errs = errors.Join(errs, err)
		errorList.Error(err.Error())
	}

	return errs
}

func getNamespace(path string) string {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}

func validateOpenAPIDir(path string) error {
	openAPIDir := filepath.Join(path, "openapi")
	if _, err := os.Stat(openAPIDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("OpenAPI dir does not exist")
		}

		return fmt.Errorf("failed to access OpenAPI dir: %w", err)
	}

	var errs error

	if _, err := os.Stat(filepath.Join(openAPIDir, "values.yaml")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errs = errors.Join(errs, fmt.Errorf("OpenAPI dir does not contain values.yaml"))
		} else {
			errs = errors.Join(errs, fmt.Errorf("failed to access OpenAPI values.yaml: %w", err))
		}
	}

	if _, err := os.Stat(filepath.Join(openAPIDir, "config-values.yaml")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errs = errors.Join(errs, fmt.Errorf("OpenAPI dir does not contain config-values.yaml"))
		} else {
			errs = errors.Join(errs, fmt.Errorf("failed to access OpenAPI config-values.yaml: %w", err))
		}
	}

	return errs
}
