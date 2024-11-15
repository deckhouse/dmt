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

package monitoring

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/pkg/errors"
)

func dirExists(moduleName, modulePath string, path ...string) (bool, *errors.LintRuleError) {
	searchPath := filepath.Join(append([]string{modulePath}, path...)...)
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.NewLintRuleError(
			ID,
			moduleName,
			modulePath,
			path,
			"%v", err.Error(),
		)
	}
	return info.IsDir(), nil
}

func MonitoringModuleRule(moduleName, modulePath, moduleNamespace string) *errors.LintRuleError {
	if slices.Contains(Cfg.SkipModuleChecks, moduleName) {
		return nil
	}

	folderEx, lerr := dirExists(moduleName, modulePath, "monitoring")
	if lerr != nil {
		return lerr
	}

	if !folderEx {
		return nil
	}

	rulesEx, lerr := dirExists(moduleName, modulePath, "monitoring", "prometheus-rules")
	if lerr != nil {
		return lerr
	}

	dashboardsEx, lerr := dirExists(moduleName, modulePath, "monitoring", "grafana-dashboards")
	if lerr != nil {
		return lerr
	}

	searchingFilePath := filepath.Join(modulePath, "templates", "monitoring.yaml")
	info, _ := os.Stat(searchingFilePath)
	if info == nil {
		return errors.NewLintRuleError(
			ID,
			moduleName,
			modulePath,
			searchingFilePath,
			"Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file",
		)
	}

	content, err := os.ReadFile(searchingFilePath)
	if err != nil {
		return errors.NewLintRuleError(
			ID,
			moduleName,
			modulePath,
			searchingFilePath,
			"%v",
			err.Error(),
		)
	}

	desiredContentBuilder := strings.Builder{}
	if dashboardsEx {
		desiredContentBuilder.WriteString("{{- include \"helm_lib_grafana_dashboard_definitions\" . }}\n")
	}

	if rulesEx {
		desiredContentBuilder.WriteString(
			"{{- include \"helm_lib_prometheus_rules\" (list . %q) }}\n",
		)
	}

	var res bool
	for _, namespace := range []string{moduleNamespace, "d8-system", "d8-monitoring"} {
		var desiredContent string
		if rulesEx {
			desiredContent = fmt.Sprintf(desiredContentBuilder.String(), namespace)
		} else {
			desiredContent = desiredContentBuilder.String()
		}
		res = res || desiredContent == string(content)
	}

	if !res {
		return errors.NewLintRuleError(
			ID,
			searchingFilePath,
			modulePath,
			nil,
			"The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s",
			fmt.Sprintf(desiredContentBuilder.String(), "YOUR NAMESPACE TO DEPLOY RULES: d8-monitoring, d8-system or module namespaces"),
			string(content),
		)
	}

	return nil
}
