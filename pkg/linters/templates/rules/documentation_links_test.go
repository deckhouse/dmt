/*
Copyright 2026 Flant JSC

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
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// documentationLinksMockModule builds a Module mock rooted at modulePath. The
// rule reads source files only, so the rendered storage is never requested.
func documentationLinksMockModule(t *testing.T, modulePath string) *mocks.ModuleMock {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(modulePath)

	return m
}

func TestDocumentationLinksRule_Check(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]string
		exclude      []pkg.StringRuleExclude
		wantCount    int
		wantContains []string
		wantLines    []int
	}{
		{
			name: "flags a public link in a plain yaml rules file and suggests renaming it",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.yaml": `- name: group
  rules:
  - alert: A
    annotations:
      description: See [docs](https://deckhouse.io/modules/foo/faq.html#bar).
`,
			},
			wantCount: 1,
			wantContains: []string{
				`"https://deckhouse.io/modules/foo/faq.html#bar"`,
				`{{ include "helm_lib_module_documentation_uri" (list . "/modules/foo/faq.html#bar") }}`,
				"rename it to .tpl",
			},
			wantLines: []int{5},
		},
		{
			name: "flags deckhouse.ru and links in tpl rules without the rename hint",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.tpl": `- name: group
  rules:
  - alert: A
    annotations:
      runbook_url: https://deckhouse.ru/products/kubernetes-platform/documentation/v1/
`,
			},
			wantCount:    1,
			wantContains: []string{`(list . "/products/kubernetes-platform/documentation/v1/")`},
			wantLines:    []int{5},
		},
		{
			name: "flags links in json dashboards",
			files: map[string]string{
				"monitoring/grafana-dashboards/main/dashboard.json": `{
  "panels": [
    {"type": "text", "options": {"content": "[docs](https://deckhouse.io/modules/foo/)"}}
  ]
}
`,
			},
			wantCount: 1,
			wantLines: []int{3},
		},
		{
			name: "flags alert resources declared in templates",
			files: map[string]string{
				"templates/rules.yaml": `apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityMetricsRulesGroup
spec:
  rules:
  - alert: A
    annotations:
      runbook_url: https://deckhouse.io/modules/foo/alerts.html#a
`,
			},
			wantCount: 1,
			wantLines: []int{7},
		},
		{
			name: "flags propagated alert rules and dashboards declared in templates",
			files: map[string]string{
				"templates/propagated-rules.yaml": `apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityPropagatedMetricsRulesGroup
spec:
  rules:
  - alert: A
    annotations:
      runbook_url: https://deckhouse.io/modules/foo/alerts.html#a
`,
				"templates/dashboard.yaml": `apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
spec:
  definition: |
    {"links": [{"url": "https://deckhouse.io/modules/foo/"}]}
`,
			},
			wantCount: 2,
		},
		{
			name: "ignores logs rules groups: they hold recording rules only, without annotations",
			files: map[string]string{
				"templates/logs-rules.yaml": `apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityLogsRulesGroup
spec:
  rules:
  - record: foo:count
    expr: count_over_time({app="foo"}[5m])
    labels:
      docs: https://deckhouse.io/modules/foo/
`,
			},
			wantCount: 0,
		},
		{
			name: "ignores templates that are not alerts or dashboards",
			files: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
data:
  url: https://deckhouse.io/modules/foo/
`,
			},
			wantCount: 0,
		},
		{
			name: "ignores links built from the in-cluster documentation helper",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.tpl": `- name: group
  rules:
  - alert: A
    annotations:
      runbook_url: {{ include "helm_lib_module_documentation_uri" (list . "/modules/foo/faq.html") }}
`,
			},
			wantCount: 0,
		},
		{
			name: "accepts an inline fallback in the else branch of publicDomainTemplate",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.tpl": `- name: group
  rules:
  - alert: A
    annotations:
      description: |
        Refer to the [docs]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/modules/foo/examples.html).
`,
			},
			wantCount: 0,
		},
		{
			name: "accepts block fallbacks and still flags a link outside of them",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.tpl": `- name: group
  rules:
  - alert: A
    annotations:
    {{- if .Values.global.modules.publicDomainTemplate }}
      summary: The {{` + "`{{ $labels.node }}`" + `}} Node, see [docs]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/foo/).
    {{- else }}
      summary: The {{` + "`{{ $labels.node }}`" + `}} Node, see [docs](https://deckhouse.io/modules/foo/).
    {{- end }}
{{ if not .Values.global.modules.publicDomainTemplate }}
      description: See https://deckhouse.io/modules/foo/faq.html.
{{ end }}
      runbook_url: https://deckhouse.io/modules/foo/runbook.html
`,
			},
			wantCount: 1,
			wantLines: []int{13},
		},
		{
			name: "a publicDomainTemplate condition in a plain yaml file is not rendered, so it is not a fallback",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.yaml": `- name: group
  rules:
  - alert: A
    annotations:
      description: "{{ if .Values.global.modules.publicDomainTemplate }}x{{ else }}https://deckhouse.io/modules/foo/{{ end }}"
`,
			},
			wantCount: 1,
		},
		{
			name: "ignores hostnames that only start with deckhouse.io",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.yaml": `- name: group
  rules:
  - alert: A
    annotations:
      description: https://deckhouse.io.example.com/ and https://deckhouse.ru-mirror.example.com/
`,
			},
			wantCount: 0,
		},
		{
			name: "an excluded file is skipped",
			files: map[string]string{
				"monitoring/prometheus-rules/alerts.yaml": `- name: group
  rules:
  - alert: A
    annotations:
      description: https://deckhouse.io/modules/foo/
`,
			},
			exclude:   []pkg.StringRuleExclude{"monitoring/prometheus-rules/alerts.yaml"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := writeTemplatesModule(t, tt.files)

			errorList := errors.NewLintRuleErrorsList()
			NewDocumentationLinksRule(tt.exclude, nil, documentationLinksMockModule(t, modulePath), errorList).Check(t.Context())

			errs := errorList.GetErrors()
			require.Len(t, errs, tt.wantCount, "%+v", errs)

			for _, want := range tt.wantContains {
				found := false

				for i := range errs {
					if containsStr(errs[i].Text, want) {
						found = true
						break
					}
				}

				require.Truef(t, found, "expected a finding containing %q, got %+v", want, errs)
			}

			for i, wantLine := range tt.wantLines {
				require.Equalf(t, wantLine, errs[i].LineNumber, "unexpected line for finding %d: %s", i, errs[i].Text)
			}
		})
	}
}

func TestDocumentationLinksRule_DirectoryExclusion(t *testing.T) {
	modulePath := writeTemplatesModule(t, map[string]string{
		"monitoring/grafana-dashboards/legacy/dashboard.json": `{"links": [{"url": "https://deckhouse.io/modules/foo/"}]}`,
	})

	errorList := errors.NewLintRuleErrorsList()
	NewDocumentationLinksRule(
		nil,
		[]pkg.DirectoryRuleExclude{"monitoring/grafana-dashboards/legacy/"},
		documentationLinksMockModule(t, modulePath),
		errorList,
	).Check(t.Context())

	require.Empty(t, errorList.GetErrors())
}
