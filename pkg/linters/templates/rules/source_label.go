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
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/tidwall/gjson"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	SourceLabelRuleName = "source-label"
)

type SourceLabelRule struct {
	pkg.RuleMeta
	pkg.BoolRule
	recordingRuleNames map[string]struct{}
	allowedMetrics     map[string]struct{}
}

func NewSourceLabelRule(cfg *pkg.TemplatesLinterConfig) *SourceLabelRule {
	var exclude bool
	allowedMetrics := make(map[string]struct{})
	recordNames := make(map[string]struct{})
	if cfg != nil {
		exclude = cfg.SourceLabelSettings.Disable
		for _, m := range cfg.SourceLabelSettings.AllowedMetrics {
			allowedMetrics[m] = struct{}{}
		}
		if cfg.SourceLabelSettings.RecordingRuleNames != nil {
			recordNames = cfg.SourceLabelSettings.RecordingRuleNames
		}
	}

	return &SourceLabelRule{
		RuleMeta: pkg.RuleMeta{
			Name: SourceLabelRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: exclude,
		},
		recordingRuleNames: recordNames,
		allowedMetrics:     allowedMetrics,
	}
}

func (r *SourceLabelRule) SourceLabelCheck(m pkg.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	if object.Unstructured.GetKind() != "PrometheusRule" {
		return
	}

	ispec, ok := object.Unstructured.Object["spec"]
	if !ok {
		return
	}
	spec, ok := ispec.(map[string]any)
	if !ok {
		return
	}

	specBytes, err := yaml.Marshal(spec)
	if err != nil {
		return
	}

	type rule struct {
		Record string `yaml:"record,omitempty"`
		Alert  string `yaml:"alert,omitempty"`
		Expr   string `yaml:"expr"`
	}
	type ruleGroup struct {
		Name  string `yaml:"name"`
		Rules []rule `yaml:"rules"`
	}
	type ruleGroups struct {
		Groups []ruleGroup `yaml:"groups"`
	}

	var groups ruleGroups
	if err := yaml.Unmarshal(specBytes, &groups); err != nil {
		return
	}

	for _, group := range groups.Groups {
		for _, rl := range group.Rules {
			if rl.Expr == "" {
				continue
			}
			ruleName := rl.Alert
			if ruleName == "" {
				ruleName = rl.Record
			}
			r.checkExpr(rl.Expr, ruleName, group.Name, object.GetPath(), errorList)
		}
	}
}

func (r *SourceLabelRule) checkExpr(expr, ruleName, groupName, filePath string, errorList *errors.LintRuleErrorsList) {
	ast, err := parser.ParseExpr(expr)
	if err != nil {
		return
	}

	parser.Inspect(ast, func(node parser.Node, _ []parser.Node) error {
		vs, ok := node.(*parser.VectorSelector)
		if !ok {
			return nil
		}

		metricName := vs.Name
		if metricName == "" {
			for _, m := range vs.LabelMatchers {
				if m.Name == labels.MetricName && m.Type == labels.MatchEqual {
					metricName = m.Value
					break
				}
			}
		}

		if metricName == "" {
			return nil
		}

		if _, ok := r.recordingRuleNames[metricName]; ok {
			return nil
		}
		if _, ok := r.allowedMetrics[metricName]; ok {
			return nil
		}

		hasSourceLabel := false
		for _, m := range vs.LabelMatchers {
			if m.Name == "source" && m.Type == labels.MatchEqual && m.Value == "deckhouse" {
				hasSourceLabel = true
				break
			}
		}

		if !hasSourceLabel {
			errorList.WithFilePath(filePath).
				Errorf("metric '%s' in rule '%s' (group '%s') must have source=\"deckhouse\" selector",
					metricName, ruleName, groupName)
		}

		return nil
	})
}

var (
	grafanaBuiltinVarRe = regexp.MustCompile(`\$__\w+`)
	grafanaVarBracesRe  = regexp.MustCompile(`\$\{(\w+)(?::[^}]*)?\}`)
	grafanaVarSimpleRe  = regexp.MustCompile(`\$([a-zA-Z_]\w*)`)
)

func sanitizeGrafanaExpr(expr string) string {
	result := grafanaBuiltinVarRe.ReplaceAllString(expr, "5m")
	result = grafanaVarBracesRe.ReplaceAllString(result, "__placeholder__")
	result = grafanaVarSimpleRe.ReplaceAllStringFunc(result, func(match string) string {
		name := match[1:]
		if name == "source" {
			return match
		}
		return "__placeholder__"
	})
	return result
}

func (r *SourceLabelRule) SourceLabelCheckDashboards(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	errorList = errorList.WithRule(r.GetName())

	searchPath := filepath.Join(m.GetPath(), "monitoring", "grafana-dashboards")
	entries := fsutils.GetFiles(searchPath, true, fsutils.FilterFileByExtensions(".json", ".tpl"))

	for _, entry := range entries {
		r.checkDashboardFile(entry, errorList)
	}
}

func (r *SourceLabelRule) checkDashboardFile(filePath string, errorList *errors.LintRuleErrorsList) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		errorList.WithFilePath(filePath).Errorf("failed to read dashboard file: %s", err)
		return
	}

	dashboard := gjson.ParseBytes(content)
	if !dashboard.IsObject() {
		return
	}

	panels := r.extractDashboardPanels(&dashboard)
	for _, panel := range panels {
		r.checkPanel(&panel, filePath, errorList)
	}

	r.checkTemplateVariables(&dashboard, filePath, errorList)
}

func (r *SourceLabelRule) checkPanel(panel *gjson.Result, filePath string, errorList *errors.LintRuleErrorsList) {
	panelTitle := panel.Get("title").String()
	if panelTitle == "" {
		panelTitle = "unnamed"
	}

	targets := panel.Get("targets").Array()
	for _, target := range targets {
		expr := target.Get("expr").String()
		if expr == "" {
			continue
		}
		sanitized := sanitizeGrafanaExpr(expr)
		r.checkExpr(sanitized, fmt.Sprintf("panel '%s'", panelTitle), "dashboard", filePath, errorList)
	}
}

func (r *SourceLabelRule) checkTemplateVariables(dashboard *gjson.Result, filePath string, errorList *errors.LintRuleErrorsList) {
	templating := dashboard.Get("templating.list")
	if !templating.Exists() || !templating.IsArray() {
		return
	}

	for _, tmpl := range templating.Array() {
		if tmpl.Get("type").String() != "query" {
			continue
		}

		query := tmpl.Get("definition").String()
		if query == "" {
			query = tmpl.Get("query").String()
		}
		if query == "" {
			continue
		}

		sanitized := sanitizeGrafanaExpr(query)
		tmplName := tmpl.Get("name").String()
		r.checkExpr(sanitized, fmt.Sprintf("template variable '%s'", tmplName), "dashboard", filePath, errorList)
	}
}

func (r *SourceLabelRule) extractDashboardPanels(dashboard *gjson.Result) []gjson.Result {
	panels := make([]gjson.Result, 0)

	rows := dashboard.Get("rows").Array()
	for _, row := range rows {
		rowPanels := row.Get("panels").Array()
		panels = append(panels, rowPanels...)
	}

	directPanels := dashboard.Get("panels").Array()
	for _, panel := range directPanels {
		panelType := panel.Get("type").String()
		if panelType == "row" {
			rowPanels := panel.Get("panels").Array()
			panels = append(panels, rowPanels...)
		} else {
			panels = append(panels, panel)
		}
	}

	return panels
}
