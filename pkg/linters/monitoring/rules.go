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
	"strings"

	"github.com/deckhouse/dmt/pkg/errors"
)

func dirExists(modulePath string, path ...string) error {
	searchPath := filepath.Join(append([]string{modulePath}, path...)...)
	_, err := os.Stat(searchPath)
	if err != nil {
		return err
	}
	return nil
}

func (l *Monitoring) checkMonitoringRules(moduleName, modulePath, moduleNamespace string) {
	errorList := l.ErrorList.WithModule(moduleName)

	if l.cfg.MonitoringRules != nil && !*l.cfg.MonitoringRules {
		return
	}

	if err := dirExists(modulePath, "monitoring"); err != nil {
		if os.IsNotExist(err) {
			return
		}

		errorList.Errorf("reading the 'monitoring' folder failed: %s", err)
		return
	}

	monitoringFilePath := filepath.Join(modulePath, "templates", "monitoring.yaml")
	if info, _ := os.Stat(monitoringFilePath); info == nil {
		errorList.WithFilePath(monitoringFilePath).Error("Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file")
		return
	}

	content, err := os.ReadFile(monitoringFilePath)
	if err != nil {
		errorList.WithFilePath(monitoringFilePath).Errorf("Cannot read 'templates/monitoring.yaml' file: %s", err)
		return
	}

	validatePrometheusRules(modulePath, moduleNamespace, monitoringFilePath, string(content), errorList)

	validationGrafanaDashboards(modulePath, moduleNamespace, monitoringFilePath, string(content), errorList)
}

func validatePrometheusRules(modulePath, moduleNamespace, monitoringFilePath, content string, errList *errors.LintRuleErrorsList) {
	searchPath := filepath.Join(modulePath, "monitoring", "prometheus-rules")
	_, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		errList.Errorf("reading the 'monitoring/prometheus-rules' folder failed: %s", err)
		return
	}

	desiredContent := "{{- include \"helm_lib_prometheus_rules\" (list . %q) }}"

	if !isContentMatching(content, desiredContent, moduleNamespace, true) {
		errList.WithFilePath(monitoringFilePath).Errorf(
			"The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s",
			fmt.Sprintf(desiredContent, "YOUR NAMESPACE TO DEPLOY RULES: d8-monitoring, d8-system or module namespaces"),
			content,
		)
		return
	}
}

func validationGrafanaDashboards(modulePath, moduleNamespace, monitoringFilePath, content string, errList *errors.LintRuleErrorsList) {
	searchPath := filepath.Join(modulePath, "monitoring", "grafana-dashboards")
	_, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		errList.Errorf("reading the 'monitoring/grafana-dashboards' folder failed: %s", err)
		return
	}

	desiredContent := "{{- include \"helm_lib_grafana_dashboard_definitions\" . }}"

	if !isContentMatching(content, desiredContent, moduleNamespace, false) {
		errList.WithFilePath(monitoringFilePath).Errorf(
			"The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s",
			desiredContent,
			content,
		)
		return
	}
}

func isContentMatching(content, desiredContent, moduleNamespace string, rulesEx bool) bool {
	for _, namespace := range []string{moduleNamespace, "d8-system", "d8-monitoring"} {
		checkContent := desiredContent
		if rulesEx {
			checkContent = fmt.Sprintf(desiredContent, namespace)
		}
		if strings.Contains(content, checkContent) {
			return true
		}
	}
	return false
}
