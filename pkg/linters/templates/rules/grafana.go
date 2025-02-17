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

package rules

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"

	"os"
	"path/filepath"
	"strings"
)

const (
	GrafanaRuleName = "grafana"
)

func NewGrafanaRule() *GrafanaRule {
	return &GrafanaRule{
		RuleMeta: pkg.RuleMeta{
			Name: GrafanaRuleName,
		},
	}
}

type GrafanaRule struct {
	pkg.RuleMeta
}

func (r *GrafanaRule) ValidationGrafanaDashboards(m *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	monitoringFilePath := filepath.Join(m.GetPath(), "templates", "monitoring.yaml")
	if info, _ := os.Stat(monitoringFilePath); info == nil {
		errorList.WithFilePath(monitoringFilePath).
			Error("Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file")
		return
	}

	content, err := os.ReadFile(monitoringFilePath)
	if err != nil {
		errorList.WithFilePath(monitoringFilePath).
			Errorf("Cannot read 'templates/monitoring.yaml' file: %s", err)
		return
	}

	searchPath := filepath.Join(m.GetPath(), "monitoring", "grafana-dashboards")
	_, err = os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}

		errorList.Errorf("reading the 'monitoring/grafana-dashboards' folder failed: %s", err)

		return
	}

	desiredContent := `{{- include "helm_lib_grafana_dashboard_definitions" . }}`

	if isContentMatching(string(content), desiredContent, m.GetNamespace(), false) {
		return
	}
	if strings.Contains(string(content), `include "helm_lib_grafana_dashboard_definitions_recursion" (list .`) {
		return
	}

	errorList.WithFilePath(monitoringFilePath).
		Errorf("The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s", desiredContent, string(content))
}
