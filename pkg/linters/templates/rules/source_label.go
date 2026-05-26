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
	"strings"

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

// prometheusSyntheticMetrics contains Prometheus built-in metrics that are generated
// internally by the engine and never receive scrape-time labels like source="deckhouse".
// This set is intentionally separate from allowedMetrics (per-module config) and
// recordingRuleNames (computed at runtime) because these metrics are universally
// synthetic regardless of module or deployment.
var prometheusSyntheticMetrics = map[string]struct{}{
	"ALERTS":           {},
	"ALERTS_FOR_STATE": {},
}

type SourceLabelRule struct {
	pkg.RuleMeta
	pkg.BoolRule
	recordingRuleNames map[string]struct{}
	allowedMetrics     []*regexp.Regexp
}

// globToRegexp converts a simple glob pattern (supporting * and ?) to a regexp.
// Plain strings without wildcards are compiled as ^exact_name$, behaving like exact match.
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")

	for _, ch := range pattern {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}

	b.WriteString("$")

	return regexp.Compile(b.String())
}

func NewSourceLabelRule(cfg *pkg.TemplatesLinterConfig) *SourceLabelRule {
	var exclude bool

	var allowedMetrics []*regexp.Regexp

	recordNames := make(map[string]struct{})

	if cfg != nil {
		exclude = cfg.SourceLabelSettings.Disable

		for _, m := range cfg.SourceLabelSettings.AllowedMetrics {
			if re, err := globToRegexp(m); err == nil {
				allowedMetrics = append(allowedMetrics, re)
			}
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

		if metricName == "" || metricName == "__placeholder__" {
			return nil
		}

		if _, ok := r.recordingRuleNames[metricName]; ok {
			return nil
		}

		if r.isAllowedMetric(metricName) {
			return nil
		}

		if _, ok := prometheusSyntheticMetrics[metricName]; ok {
			return nil
		}

		hasSourceLabel := false

		for _, m := range vs.LabelMatchers {
			if m.Name == "source" && m.Type == labels.MatchEqual &&
				(m.Value == "deckhouse" || strings.HasPrefix(m.Value, "$")) {
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

func (r *SourceLabelRule) isAllowedMetric(metricName string) bool {
	for _, re := range r.allowedMetrics {
		if re.MatchString(metricName) {
			return true
		}
	}

	return false
}

var (
	grafanaBuiltinVarRe = regexp.MustCompile(`\$__\w+`)
	grafanaVarBracesRe  = regexp.MustCompile(`\$\{(\w+)(?::[^}]*)?\}`)
	grafanaVarSimpleRe  = regexp.MustCompile(`\$([a-zA-Z_]\w*)`)
)

func sanitizeGrafanaExpr(expr string) string {
	result := grafanaBuiltinVarRe.ReplaceAllString(expr, "5m")
	result = grafanaVarBracesRe.ReplaceAllStringFunc(result, func(match string) string {
		sub := grafanaVarBracesRe.FindStringSubmatch(match)
		if len(sub) > 1 && sub[1] == "source" {
			return match
		}

		return "__placeholder__"
	})
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

func isPrometheusDataSource(obj *gjson.Result) bool {
	ds := obj.Get("datasource")
	if !ds.Exists() {
		return true
	}

	if ds.Type == gjson.String {
		return strings.Contains(strings.ToLower(ds.String()), "prometheus")
	}

	dsType := ds.Get("type").String()

	return dsType == "" || dsType == "prometheus"
}

func (r *SourceLabelRule) checkPanel(panel *gjson.Result, filePath string, errorList *errors.LintRuleErrorsList) {
	if !isPrometheusDataSource(panel) {
		return
	}

	panelTitle := panel.Get("title").String()
	if panelTitle == "" {
		panelTitle = "unnamed"
	}

	targets := panel.Get("targets").Array()
	for _, target := range targets {
		if !isPrometheusDataSource(&target) {
			continue
		}

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

		if !isPrometheusDataSource(&tmpl) {
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
	panels = append(panels, collectPanelsRecursive(directPanels)...)

	return panels
}

func collectPanelsRecursive(items []gjson.Result) []gjson.Result {
	var result []gjson.Result

	for _, item := range items {
		if item.Get("type").String() == "row" {
			nested := item.Get("panels").Array()
			result = append(result, collectPanelsRecursive(nested)...)
		} else {
			result = append(result, item)
		}
	}

	return result
}
