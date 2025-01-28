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

func dirExists(moduleName, modulePath string, path ...string) (bool, *errors.LintRuleErrorsList) {
	result := errors.NewLinterRuleList(ID, moduleName)
	searchPath := filepath.Join(append([]string{modulePath}, path...)...)
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, result.WithObjectID(modulePath).AddF("%v", err.Error())
	}
	return info.IsDir(), nil
}

func MonitoringModuleRule(moduleName, modulePath, moduleNamespace string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, moduleName)
	if slices.Contains(Cfg.SkipModuleChecks, moduleName) {
		return nil
	}

	if exists, lerr := dirExists(moduleName, modulePath, "monitoring"); lerr != nil || !exists {
		return lerr
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
	if info, _ := os.Stat(searchingFilePath); info == nil {
		return result.WithObjectID(modulePath).
			AddWithValue(searchingFilePath, "Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file")
	}

	content, err := os.ReadFile(searchingFilePath)
	if err != nil {
		return result.WithObjectID(modulePath).AddWithValue(
			searchingFilePath,
			"%v",
			err.Error(),
		)
	}

	desiredContent := buildDesiredContent(dashboardsEx, rulesEx)
	if !isContentMatching(string(content), desiredContent, moduleNamespace, rulesEx) {
		return result.WithObjectID(modulePath).AddF(
			"The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s",
			fmt.Sprintf(desiredContent, "YOUR NAMESPACE TO DEPLOY RULES: d8-monitoring, d8-system or module namespaces"),
			string(content),
		)
	}

	return nil
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
