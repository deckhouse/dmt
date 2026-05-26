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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestCheckExprWithSourceLabel(t *testing.T) {
	tests := []struct {
		name           string
		expr           string
		recordNames    map[string]struct{}
		allowedMetrics []string
		expectedErrors int
	}{
		{
			name:           "metric with source=deckhouse has no errors",
			expr:           `kube_pod_info{source="deckhouse"}`,
			expectedErrors: 0,
		},
		{
			name:           "metric without source selector produces error",
			expr:           `kube_pod_info`,
			expectedErrors: 1,
		},
		{
			name:           "recording rule name is allowed without source",
			expr:           `my_recording_rule`,
			recordNames:    map[string]struct{}{"my_recording_rule": {}},
			expectedErrors: 0,
		},
		{
			name:           "binary expr - one metric with source, one without",
			expr:           `a{source="deckhouse"} * on() b`,
			expectedErrors: 1,
		},
		{
			name:           "exact allowed metric does not produce error",
			expr:           `allowed_metric`,
			allowedMetrics: []string{"allowed_metric"},
			expectedErrors: 0,
		},
		{
			name:           "glob pattern matches metric prefix",
			expr:           `coredns_panics_total + coredns_dns_requests_total`,
			allowedMetrics: []string{"coredns_*"},
			expectedErrors: 0,
		},
		{
			name:           "glob pattern does not match unrelated metric",
			expr:           `kube_pod_info`,
			allowedMetrics: []string{"coredns_*"},
			expectedErrors: 1,
		},
		{
			name:           "glob pattern with question mark",
			expr:           `metric_v1_total + metric_v2_total`,
			allowedMetrics: []string{"metric_v?_total"},
			expectedErrors: 0,
		},
		{
			name:           "mixed exact and glob",
			expr:           `exact_metric + coredns_cache_hits_total + unknown_metric`,
			allowedMetrics: []string{"exact_metric", "coredns_*"},
			expectedErrors: 1,
		},
		{
			name:           "ALERTS synthetic metric is allowed without source",
			expr:           `ALERTS{alertname="SomeAlert"}`,
			expectedErrors: 0,
		},
		{
			name:           "ALERTS_FOR_STATE synthetic metric is allowed without source",
			expr:           `ALERTS_FOR_STATE{alertname="SomeAlert"}`,
			expectedErrors: 0,
		},
		{
			name:           "ALERTS in complex expr does not produce error",
			expr:           `ALERTS{alertname="KubeQuotaExceeded"} == 1 and on(namespace) kube_pod_info{source="deckhouse"}`,
			expectedErrors: 0,
		},
		{
			name:           "ALERTS without source but regular metric also without source",
			expr:           `ALERTS{alertname="X"} * on() some_metric`,
			expectedErrors: 1,
		},
		{
			name:           "placeholder metric name is skipped",
			expr:           `__placeholder__{some="label"}`,
			expectedErrors: 0,
		},
		{
			name:           "placeholder embedded in metric name is skipped",
			expr:           `__placeholder___requests_total{some="label"}`,
			expectedErrors: 0,
		},
		{
			name:           "invalid PromQL is silently skipped",
			expr:           `label_values(kube_pod_info, namespace)`,
			expectedErrors: 0,
		},
		{
			name:           "expression with $source variable is accepted",
			expr:           `m{source="$source"}`,
			expectedErrors: 0,
		},
		{
			name:           "expression with ${source} variable is accepted",
			expr:           `m{source="${source}"}`,
			expectedErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recordNames := tt.recordNames
			if recordNames == nil {
				recordNames = make(map[string]struct{})
			}

			compiled := make([]*regexp.Regexp, 0, len(tt.allowedMetrics))

			for _, m := range tt.allowedMetrics {
				re, err := globToRegexp(m)
				assert.NoError(t, err)

				compiled = append(compiled, re)
			}

			rule := &SourceLabelRule{
				recordingRuleNames: recordNames,
				allowedMetrics:     compiled,
			}

			errorList := errors.NewLintRuleErrorsList()
			rule.checkExpr(tt.expr, "test-rule", "test-group", "/test/file.yaml", errorList)

			assert.Len(t, errorList.GetErrors(), tt.expectedErrors)
		})
	}
}

func TestGlobToRegexp(t *testing.T) {
	tests := []struct {
		pattern string
		match   []string
		noMatch []string
	}{
		{
			pattern: "coredns_*",
			match:   []string{"coredns_panics_total", "coredns_dns_requests_total", "coredns_"},
			noMatch: []string{"xcoredns_foo", "kube_pod_info", "coredns"},
		},
		{
			pattern: "metric_v?_total",
			match:   []string{"metric_v1_total", "metric_vX_total"},
			noMatch: []string{"metric_v12_total", "metric_v_total"},
		},
		{
			pattern: "exact_metric",
			match:   []string{"exact_metric"},
			noMatch: []string{"exact_metric_extra", "xexact_metric", "exact_metricx"},
		},
		{
			pattern: "ingress_nginx_*_responses_*",
			match:   []string{"ingress_nginx_detail_responses_total", "ingress_nginx_overall_responses_5xx"},
			noMatch: []string{"ingress_nginx_connections", "nginx_responses_total"},
		},
		{
			pattern: "*.total",
			match:   []string{"foo.total", "bar_baz.total"},
			noMatch: []string{"foo_total", "total"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			re, err := globToRegexp(tt.pattern)
			assert.NoError(t, err)

			for _, s := range tt.match {
				assert.True(t, re.MatchString(s), "expected %q to match pattern %q", s, tt.pattern)
			}

			for _, s := range tt.noMatch {
				assert.False(t, re.MatchString(s), "expected %q NOT to match pattern %q", s, tt.pattern)
			}
		})
	}
}

func TestIsPrometheusDataSource(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected bool
	}{
		{
			name:     "no datasource field defaults to prometheus",
			json:     `{"title": "panel"}`,
			expected: true,
		},
		{
			name:     "datasource type prometheus",
			json:     `{"datasource": {"type": "prometheus", "uid": "$ds"}}`,
			expected: true,
		},
		{
			name:     "datasource type empty defaults to prometheus",
			json:     `{"datasource": {"uid": "$ds"}}`,
			expected: true,
		},
		{
			name:     "datasource is plain string Graphite",
			json:     `{"datasource": "Graphite"}`,
			expected: false,
		},
		{
			name:     "datasource is plain string with prometheus in name",
			json:     `{"datasource": "$ds_prometheus"}`,
			expected: true,
		},
		{
			name:     "datasource is plain string Prometheus capitalized",
			json:     `{"datasource": "Prometheus"}`,
			expected: true,
		},
		{
			name:     "datasource type loki",
			json:     `{"datasource": {"type": "loki", "uid": "$ds_loki"}}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gjson.Parse(tt.json)
			assert.Equal(t, tt.expected, isPrometheusDataSource(&result))
		})
	}
}

func TestCollectPanelsRecursive(t *testing.T) {
	tests := []struct {
		name           string
		json           string
		expectedTitles []string
	}{
		{
			name: "flat panels",
			json: `[
				{"type": "graph", "title": "A"},
				{"type": "stat", "title": "B"}
			]`,
			expectedTitles: []string{"A", "B"},
		},
		{
			name: "one level of row nesting",
			json: `[
				{"type": "row", "panels": [
					{"type": "graph", "title": "A"}
				]},
				{"type": "stat", "title": "B"}
			]`,
			expectedTitles: []string{"A", "B"},
		},
		{
			name: "two levels of row nesting",
			json: `[
				{"type": "row", "panels": [
					{"type": "row", "panels": [
						{"type": "graph", "title": "deep"}
					]}
				]}
			]`,
			expectedTitles: []string{"deep"},
		},
		{
			name: "three levels of row nesting",
			json: `[
				{"type": "row", "panels": [
					{"type": "row", "panels": [
						{"type": "row", "panels": [
							{"type": "graph", "title": "very-deep"}
						]}
					]}
				]}
			]`,
			expectedTitles: []string{"very-deep"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := gjson.Parse(tt.json).Array()
			panels := collectPanelsRecursive(items)

			titles := make([]string, 0, len(panels))
			for _, p := range panels {
				titles = append(titles, p.Get("title").String())
			}

			assert.Equal(t, tt.expectedTitles, titles)
		})
	}
}

func TestSanitizeGrafanaExpr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replaces $__rate_interval with 5m",
			input:    `rate(m{source="deckhouse"}[$__rate_interval])`,
			expected: `rate(m{source="deckhouse"}[5m])`,
		},
		{
			name:     "replaces $namespace with __placeholder__",
			input:    `m{namespace="$namespace"}`,
			expected: `m{namespace="__placeholder__"}`,
		},
		{
			name:     "replaces ${var:regex} with __placeholder__",
			input:    `m{var="${var:regex}"}`,
			expected: `m{var="__placeholder__"}`,
		},
		{
			name:     "does not replace $source",
			input:    `m{source="$source"}`,
			expected: `m{source="$source"}`,
		},
		{
			name:     "replaces $__range with 5m",
			input:    `increase(m[$__range])`,
			expected: `increase(m[5m])`,
		},
		{
			name:     "does not replace ${source}",
			input:    `m{source="${source}"}`,
			expected: `m{source="${source}"}`,
		},
		{
			name:     "does not replace ${source:json}",
			input:    `m{source="${source:json}"}`,
			expected: `m{source="${source:json}"}`,
		},
		{
			name:     "replaces ${other_var} with __placeholder__",
			input:    `m{ns="${namespace}"}`,
			expected: `m{ns="__placeholder__"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeGrafanaExpr(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
