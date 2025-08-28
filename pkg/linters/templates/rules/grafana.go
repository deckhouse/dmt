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

package rules

import (
	"github.com/tidwall/gjson"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"

	"os"
	"path/filepath"
	"strings"
)

const (
	GrafanaRuleName = "grafana"
)

func NewGrafanaRule(cfg *config.TemplatesSettings) *GrafanaRule {
	var exclude bool
	if cfg != nil {
		exclude = cfg.GrafanaDashboards.Disable
	}
	return &GrafanaRule{
		RuleMeta: pkg.RuleMeta{
			Name: GrafanaRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: exclude,
		},
	}
}

type GrafanaRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

func (r *GrafanaRule) ValidateGrafanaDashboards(m *module.Module, errorList *errors.LintRuleErrorsList) {
	if !r.Enabled() {
		return
	}

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
	// Validate individual dashboard files
	r.validateDashboardFiles(m, errorList)

	desiredContent := `include "helm_lib_grafana_dashboard_definitions`

	if isContentMatching(content, desiredContent) {
		return
	}
	if strings.Contains(string(content), `include "helm_lib_grafana_dashboard_definitions_recursion" (list .`) {
		return
	}

	desiredContent = `{{- include "helm_lib_grafana_dashboard_definitions" . }}`
	errorList.WithFilePath(monitoringFilePath).
		Errorf("The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s", desiredContent, string(content))
}

// validateDashboardFiles validates individual grafana dashboard files
func (r *GrafanaRule) validateDashboardFiles(m *module.Module, errorList *errors.LintRuleErrorsList) {
	searchPath := filepath.Join(m.GetPath(), "monitoring", "grafana-dashboards")

	entries := fsutils.GetFiles(searchPath, true, fsutils.FilterFileByExtensions(".json", ".tpl"))

	for _, entry := range entries {
		r.validateDashboardFile(entry, errorList)
	}
}

// validateDashboardFile validates a single grafana dashboard file
func (r *GrafanaRule) validateDashboardFile(filePath string, errorList *errors.LintRuleErrorsList) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		errorList.WithFilePath(filePath).Errorf("failed to read dashboard file: %s", err)
		return
	}

	// Parse JSON content
	dashboard := gjson.ParseBytes(content)
	if !dashboard.IsObject() {
		errorList.WithFilePath(filePath).Error("dashboard file is not valid JSON")
		return
	}

	// Extract panels and templates
	panels := r.extractDashboardPanels(&dashboard)
	templates := r.extractDashboardTemplates(&dashboard)

	// Validate panels
	for _, panel := range panels {
		r.validatePanel(&panel, filePath, errorList)
	}

	// Validate templates
	r.validateTemplates(templates, filePath, errorList)
}

// extractDashboardPanels extracts all panels from dashboard including nested ones
func (*GrafanaRule) extractDashboardPanels(dashboard *gjson.Result) []gjson.Result {
	panels := make([]gjson.Result, 0)

	// Extract panels from rows
	rows := dashboard.Get("rows").Array()
	for _, row := range rows {
		rowPanels := row.Get("panels").Array()
		panels = append(panels, rowPanels...)
	}

	// Extract direct panels
	directPanels := dashboard.Get("panels").Array()
	for _, panel := range directPanels {
		panelType := panel.Get("type").String()
		if panelType == "row" {
			// Extract panels from row
			rowPanels := panel.Get("panels").Array()
			panels = append(panels, rowPanels...)
		} else {
			panels = append(panels, panel)
		}
	}

	return panels
}

// extractDashboardTemplates extracts template variables from dashboard
func (*GrafanaRule) extractDashboardTemplates(dashboard *gjson.Result) []gjson.Result {
	templating := dashboard.Get("templating")
	if !templating.Exists() {
		return []gjson.Result{}
	}

	list := templating.Get("list")
	if !list.Exists() || !list.IsArray() {
		return []gjson.Result{}
	}

	return list.Array()
}

// validatePanel validates a single panel
func (r *GrafanaRule) validatePanel(panel *gjson.Result, filePath string, errorList *errors.LintRuleErrorsList) {
	panelTitle := panel.Get("title").String()
	if panelTitle == "" {
		panelTitle = "unnamed"
	}

	// Check deprecated panel types
	r.checkDeprecatedPanelTypes(panel, panelTitle, filePath, errorList)

	// Check deprecated intervals
	r.checkDeprecatedIntervals(panel, panelTitle, filePath, errorList)

	// Check legacy alert rules
	r.checkLegacyAlertRules(panel, panelTitle, filePath, errorList)

	// Check datasource validation
	r.checkDatasourceValidation(panel, panelTitle, filePath, errorList)
}

// checkDeprecatedPanelTypes checks for deprecated panel types
func (*GrafanaRule) checkDeprecatedPanelTypes(panel *gjson.Result, panelTitle, filePath string, errorList *errors.LintRuleErrorsList) {
	panelType := panel.Get("type").String()
	deprecatedTypes := map[string]string{
		"graph":                 "timeseries",
		"flant-statusmap-panel": "state-timeline",
	}

	if replaceWith, isDeprecated := deprecatedTypes[panelType]; isDeprecated {
		errorList.WithFilePath(filePath).Errorf(
			"Panel '%s' uses deprecated type '%s', consider using '%s'",
			panelTitle, panelType, replaceWith,
		)
	}
}

// checkDeprecatedIntervals checks for deprecated intervals in panel queries
func (*GrafanaRule) checkDeprecatedIntervals(panel *gjson.Result, panelTitle, filePath string, errorList *errors.LintRuleErrorsList) {
	deprecatedIntervals := []string{"interval_rv", "interval_sx3", "interval_sx4"}
	targets := panel.Get("targets").Array()

	for _, target := range targets {
		expr := target.Get("expr").String()
		for _, deprecatedInterval := range deprecatedIntervals {
			if strings.Contains(expr, deprecatedInterval) {
				errorList.WithFilePath(filePath).Errorf(
					"Panel '%s' contains deprecated interval '%s', consider using '$__rate_interval'",
					panelTitle, deprecatedInterval,
				)
			}
		}
	}
}

// checkLegacyAlertRules checks for legacy alert rules in panels
func (*GrafanaRule) checkLegacyAlertRules(panel *gjson.Result, panelTitle, filePath string, errorList *errors.LintRuleErrorsList) {
	alertRule := panel.Get("alert")
	if alertRule.Exists() {
		alertRuleName := alertRule.Get("name").String()
		if alertRuleName == "" {
			alertRuleName = "unnamed"
		}
		errorList.WithFilePath(filePath).Errorf(
			"Panel '%s' contains legacy alert rule '%s', consider using external alertmanager",
			panelTitle, alertRuleName,
		)
	}
}

// checkDatasourceValidation checks datasource UIDs in panel targets
func (*GrafanaRule) checkDatasourceValidation(panel *gjson.Result, panelTitle, filePath string, errorList *errors.LintRuleErrorsList) {
	recommendedPrometheusUIDs := []string{"$ds_prometheus", "${ds_prometheus}"}
	targets := panel.Get("targets").Array()

	for _, target := range targets {
		datasource := target.Get("datasource")
		if !datasource.Exists() {
			continue
		}

		var uidStr string
		uid := datasource.Get("uid")
		if uid.Exists() {
			uidStr = uid.String()
		} else {
			// Legacy format - datasource UID is stored as string
			uidStr = datasource.String()
			errorList.WithFilePath(filePath).Errorf(
				"Panel '%s' uses legacy datasource format, consider resaving dashboard using newer Grafana version",
				panelTitle,
			)
		}

		// Check for hardcoded UIDs
		if !strings.HasPrefix(uidStr, "$") {
			errorList.WithFilePath(filePath).Errorf(
				"Panel '%s' contains hardcoded datasource UID '%s', consider using grafana variable of type 'Datasource'",
				panelTitle, uidStr,
			)
		}

		// Check Prometheus datasource UIDs
		datasourceType := datasource.Get("type")
		if datasourceType.Exists() && datasourceType.String() == "prometheus" {
			isRecommended := false
			for _, recommendedUID := range recommendedPrometheusUIDs {
				if uidStr == recommendedUID {
					isRecommended = true
					break
				}
			}

			if !isRecommended {
				errorList.WithFilePath(filePath).Errorf(
					"Panel '%s' datasource should be one of: %s instead of '%s'",
					panelTitle, strings.Join(recommendedPrometheusUIDs, ", "), uidStr,
				)
			}
		}
	}
}

// validateTemplates validates dashboard template variables
func (r *GrafanaRule) validateTemplates(templates []gjson.Result, filePath string, errorList *errors.LintRuleErrorsList) {
	hasPrometheusDatasourceVariable := false
	recommendedPrometheusUIDs := []string{"$ds_prometheus", "${ds_prometheus}"}

	for _, template := range templates {
		// Check for required Prometheus datasource variable
		if r.isPrometheusDatasourceTemplateVariable(&template) {
			hasPrometheusDatasourceVariable = true
		}

		// Check query variables for non-recommended datasource UIDs
		if r.isNonRecommendedPrometheusDatasourceQueryVariable(&template, recommendedPrometheusUIDs) {
			templateName := template.Get("name").String()
			errorList.WithFilePath(filePath).Errorf(
				"Dashboard variable '%s' should use one of: %s as its datasource",
				templateName, strings.Join(recommendedPrometheusUIDs, ", "),
			)
		}
	}

	// Check if required Prometheus datasource variable exists
	if !hasPrometheusDatasourceVariable {
		errorList.WithFilePath(filePath).Errorf(
			"Dashboard must contain prometheus variable with query type: 'prometheus' and name: 'ds_prometheus'",
		)
	}
}

// isPrometheusDatasourceTemplateVariable checks if template is the required Prometheus datasource variable
func (*GrafanaRule) isPrometheusDatasourceTemplateVariable(template *gjson.Result) bool {
	templateType := template.Get("type")
	if !templateType.Exists() || templateType.String() != "datasource" {
		return false
	}

	queryType := template.Get("query")
	if !queryType.Exists() || queryType.String() != "prometheus" {
		return false
	}

	templateName := template.Get("name")
	return templateName.Exists() && templateName.String() == "ds_prometheus"
}

// isNonRecommendedPrometheusDatasourceQueryVariable checks if query variable uses non-recommended datasource
func (*GrafanaRule) isNonRecommendedPrometheusDatasourceQueryVariable(template *gjson.Result, recommendedUIDs []string) bool {
	templateType := template.Get("type")
	if !templateType.Exists() || templateType.String() != "query" {
		return false
	}

	datasource := template.Get("datasource")
	if !datasource.Exists() {
		return false
	}

	datasourceType := datasource.Get("type")
	if !datasourceType.Exists() || datasourceType.String() != "prometheus" {
		return false
	}

	datasourceUID := datasource.Get("uid")
	if !datasourceUID.Exists() {
		return false
	}

	uidStr := datasourceUID.String()
	for _, recommendedUID := range recommendedUIDs {
		if uidStr == recommendedUID {
			return false
		}
	}

	return true
}
