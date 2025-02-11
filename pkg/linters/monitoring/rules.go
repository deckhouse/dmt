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

func dirExists(modulePath string, lintError *errors.Error, path ...string) bool {
	searchPath := filepath.Join(append([]string{modulePath}, path...)...)
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		lintError.WithObjectID(modulePath).Add("%v", err.Error())
		return false
	}
	return info.IsDir()
}

func MonitoringModuleRule(moduleName, modulePath, moduleNamespace string, lintError *errors.Error) {
	if slices.Contains(Cfg.SkipModuleChecks, moduleName) {
		return
	}

	if !dirExists(modulePath, lintError, "monitoring") {
		return
	}

	rulesEx := dirExists(modulePath, lintError, "monitoring", "prometheus-rules")
	dashboardsEx := dirExists(modulePath, lintError, "monitoring", "grafana-dashboards")
	searchingFilePath := filepath.Join(modulePath, "templates", "monitoring.yaml")
	if info, _ := os.Stat(searchingFilePath); info == nil {
		lintError.WithObjectID(modulePath).
			WithValue(searchingFilePath).
			Add("Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file")
		return
	}

	content, err := os.ReadFile(searchingFilePath)
	if err != nil {
		lintError.WithObjectID(modulePath).WithValue(searchingFilePath).Add("%v", err.Error())
		return
	}

	desiredContent := buildDesiredContent(dashboardsEx, rulesEx)
	if !isContentMatching(string(content), desiredContent, moduleNamespace, rulesEx) {
		lintError.WithObjectID(modulePath).Add(
			"The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s",
			fmt.Sprintf(desiredContent, "YOUR NAMESPACE TO DEPLOY RULES: d8-monitoring, d8-system or module namespaces"),
			string(content),
		)
	}
}

func buildDesiredContent(dashboardsEx, rulesEx bool) string {
	var builder strings.Builder
	if dashboardsEx {
		builder.WriteString("{{- include \"helm_lib_grafana_dashboard_definitions\" . }}\n")
	}
	if rulesEx {
		builder.WriteString("{{- include \"helm_lib_prometheus_rules\" (list . %q) }}\n")
	}
	return builder.String()
}

func isContentMatching(content, desiredContent, moduleNamespace string, rulesEx bool) bool {
	for _, namespace := range []string{moduleNamespace, "d8-system", "d8-monitoring"} {
		checkContent := desiredContent
		if rulesEx {
			checkContent = fmt.Sprintf(desiredContent, namespace)
		}
		if content == checkContent {
			return true
		}
	}
	return false
}
