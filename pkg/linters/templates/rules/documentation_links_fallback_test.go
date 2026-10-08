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
	"fmt"
	"testing"
	"text/template/parse"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	pdt       = ".Values.global.modules.publicDomainTemplate"
	hasDocs   = `has "documentation" .Values.global.enabledModules`
	inCluster = `{{ include "helm_lib_module_documentation_uri" (list . "/modules/foo/") }}`
)

// TestDocumentationLinksRule_Fallbacks checks which public links in a rendered
// rules file are reported. Every link has its own path, so the test asserts
// exactly which of them were flagged.
func TestDocumentationLinksRule_Fallbacks(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantLinks []string
	}{
		// Conditions that make a branch a fallback.
		{
			name: "else of publicDomainTemplate",
			body: "{{ if " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "then of not publicDomainTemplate",
			body: "{{ if not " + pdt + " }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "then of a parenthesized not",
			body: "{{ if (not " + pdt + ") }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "then of empty publicDomainTemplate",
			body: "{{ if empty " + pdt + " }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "then of publicDomainTemplate equal to an empty string",
			body: "{{ if eq " + pdt + ` "" }}https://deckhouse.io/a{{ end }}`,
		},
		{
			name: "else of with publicDomainTemplate",
			body: "{{ with " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "else of the documentation module check",
			body: "{{ if " + hasDocs + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "else of the documentation module check written as a pipeline",
			body: `{{ if .Values.global.enabledModules | has "documentation" }}` + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "then of a negated documentation module check",
			body: "{{ if not (" + hasDocs + ") }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "else of and over publicDomainTemplate and the documentation module",
			body: "{{ if and " + pdt + " (" + hasDocs + ") }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "else of a root variable path to publicDomainTemplate",
			body: "{{ if $.Values.global.modules.publicDomainTemplate }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "then of or over negated gates",
			body: "{{ if or (not " + pdt + ") (not (" + hasDocs + ")) }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "else of or with a gate: both operands are false there",
			body: "{{ if or " + pdt + " .Values.foo.enabled }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "multiline condition with trim markers",
			body: "{{- if and\n    " + pdt + "\n    (" + hasDocs + ") }}" + inCluster + "{{- else }}https://deckhouse.io/a{{- end }}",
		},
		{
			name: "inline fallback in the middle of a link",
			body: "[docs]({{ if " + pdt + ` }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/modules/foo/a.html)`,
		},

		// Conditions that do not make a branch a fallback.
		{
			name:      "link in the then branch of publicDomainTemplate",
			body:      "{{ if " + pdt + " }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "link in the then branch of the documentation module check",
			body:      "{{ if " + hasDocs + " }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "else of another module check",
			body:      `{{ if has "cni-cilium" .Values.global.enabledModules }}x{{ else }}https://deckhouse.io/a{{ end }}`,
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "else of and with an unrelated operand: it renders with the documentation available",
			body:      "{{ if and " + pdt + " .Values.foo.enabled }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "else of and over publicDomainTemplate and clusterIsBootstrapped: only the two gates count",
			body:      "{{ if and " + pdt + " .Values.global.clusterIsBootstrapped }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "else of a value that only contains publicDomainTemplate in its name",
			body:      "{{ if .Values.foo.publicDomainTemplate }}x{{ else }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "else of an unrelated condition",
			body:      "{{ if .Values.foo.enabled }}x{{ else }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "else of range",
			body:      "{{ range .Values.foo.items }}x{{ else }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},

		// Nesting: every end closes its own block.
		{
			name: "unrelated block nested in a fallback, link after its end",
			body: "{{ if " + pdt + " }}" + inCluster + "{{ else }}{{ if .Values.foo.enabled }}x{{ end }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "range with else nested in a fallback",
			body: "{{ if " + pdt + " }}" + inCluster + "{{ else }}{{ range .Values.foo.items }}x{{ else }}y{{ end }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "fallback nested in a fallback",
			body: "{{ if " + pdt + " }}" + inCluster + "{{ else }}{{ if " + hasDocs + " }}x{{ else }}https://deckhouse.io/a{{ end }}https://deckhouse.io/b{{ end }}",
		},
		{
			name:      "fallback nested in an unrelated block, link after the fallback ends",
			body:      "{{ if .Values.foo.enabled }}{{ if " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}https://deckhouse.io/b{{ end }}",
			wantLinks: []string{"https://deckhouse.io/b"},
		},
		{
			name:      "nested block in the then branch, link after it",
			body:      "{{ if " + pdt + " }}{{ if .Values.foo.enabled }}x{{ end }}https://deckhouse.io/a{{ else }}https://deckhouse.io/b{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "link after the fallback block",
			body:      "{{ if " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }} https://deckhouse.io/b",
			wantLinks: []string{"https://deckhouse.io/b"},
		},
		{
			name:      "fallback inside define, link after define",
			body:      `{{ define "foo.link" }}{{ if ` + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}{{ end }}https://deckhouse.io/b",
			wantLinks: []string{"https://deckhouse.io/b"},
		},
		{
			name: "three levels deep",
			body: "{{ if .Values.a }}{{ with .Values.b }}{{ if " + pdt + " }}" + inCluster +
				"{{ else }}https://deckhouse.io/a{{ end }}https://deckhouse.io/b{{ end }}https://deckhouse.io/c{{ end }}",
			wantLinks: []string{"https://deckhouse.io/b", "https://deckhouse.io/c"},
		},
		{
			name:      "a template that does not parse has no fallbacks: Helm cannot render it either",
			body:      "{{ end }}{{ if " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},

		// else if / else with chains.
		{
			name: "gate in else if: the final else is a fallback",
			body: "{{ if .Values.foo.enabled }}x{{ else if " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},
		{
			name: "gate in the first branch: every later branch is a fallback",
			body: "{{ if " + pdt + " }}" + inCluster + "{{ else if .Values.foo.enabled }}https://deckhouse.io/a{{ else }}https://deckhouse.io/b{{ end }}",
		},
		{
			name:      "unrelated first branch before the gate is not a fallback",
			body:      "{{ if .Values.foo.enabled }}https://deckhouse.io/a{{ else if " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/b{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name: "negated gate in else if",
			body: "{{ if .Values.foo.enabled }}x{{ else if not " + pdt + " }}https://deckhouse.io/a{{ else }}" + inCluster + "{{ end }}",
		},
		{
			name: "gate in else with",
			body: "{{ with .Values.foo }}x{{ else with " + pdt + " }}" + inCluster + "{{ else }}https://deckhouse.io/a{{ end }}",
		},

		// Escaped Prometheus templates and comments are not Helm actions.
		{
			name: "escaped Prometheus if/else inside a fallback",
			body: "{{ if " + pdt + " }}" + inCluster + "{{ else }}{{`{{ if $value }}`}}x{{`{{ else }}`}}https://deckhouse.io/a{{`{{ end }}`}}{{ end }}",
		},
		{
			name:      "escaped Prometheus else outside of a fallback",
			body:      "{{`{{ if $value }}`}}x{{`{{ else }}`}}https://deckhouse.io/a{{`{{ end }}`}}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "quoted Prometheus else outside of a fallback",
			body:      `{{ "{{ if $value }}" }}x{{ "{{ else }}" }}https://deckhouse.io/a{{ "{{ end }}" }}`,
			wantLinks: []string{"https://deckhouse.io/a"},
		},
		{
			name:      "else in a comment does not switch the branch",
			body:      "{{ if " + pdt + " }}{{/* {{ else }} */}}https://deckhouse.io/a{{ end }}",
			wantLinks: []string{"https://deckhouse.io/a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := writeTemplatesModule(t, map[string]string{
				"monitoring/prometheus-rules/alerts.tpl": tt.body + "\n",
			})

			errorList := errors.NewLintRuleErrorsList()
			NewDocumentationLinksRule(nil, nil, documentationLinksMockModule(t, modulePath), errorList).Check(t.Context())

			errs := errorList.GetErrors()

			links := make([]string, 0, len(errs))
			for _, e := range errs {
				links = append(links, fmt.Sprint(e.ObjectValue))
			}

			want := tt.wantLinks
			if want == nil {
				want = []string{}
			}

			require.ElementsMatch(t, want, links)
		})
	}
}

// conditionPipe parses {{ if cond }} and returns the pipeline of the condition.
func conditionPipe(t *testing.T, cond string) *parse.PipeNode {
	t.Helper()

	tree := parse.New("cond")
	tree.Mode = parse.SkipFuncCheck

	_, err := tree.Parse("{{ if "+cond+" }}{{ end }}", "", "", map[string]*parse.Tree{})
	require.NoError(t, err)

	return tree.Root.Nodes[0].(*parse.IfNode).Pipe
}

func TestDocumentationUnavailableWhen(t *testing.T) {
	tests := []struct {
		cond                string
		whenTrue, whenFalse bool
	}{
		{cond: pdt, whenFalse: true},
		{cond: "not " + pdt, whenTrue: true},
		{cond: "(not " + pdt + ")", whenTrue: true},
		{cond: "empty " + pdt, whenTrue: true},
		{cond: "eq " + pdt + ` ""`, whenTrue: true},
		{cond: `eq "" ` + pdt, whenTrue: true},
		{cond: `eq ` + pdt + ` "x"`},
		{cond: hasDocs, whenFalse: true},
		{cond: `.Values.global.enabledModules | has "documentation"`, whenFalse: true},
		{cond: `has "cni-cilium" .Values.global.enabledModules`},
		{cond: ".Values.global.clusterIsBootstrapped"},
		{cond: "$.Values.global.modules.publicDomainTemplate", whenFalse: true},
		{cond: ".Values.foo.publicDomainTemplate"},
		{cond: `has "documentation" .Values.foo`},
		{cond: `.Values.foo | has "documentation"`},
		{cond: "and " + pdt + " (" + hasDocs + ")", whenFalse: true},
		{cond: "and " + pdt + " .Values.foo"},
		{cond: "and " + pdt + " (not (" + hasDocs + "))", whenTrue: true},
		{cond: "or " + pdt + " .Values.foo", whenFalse: true},
		{cond: "or (not " + pdt + ") (not (" + hasDocs + "))", whenTrue: true},
		{cond: "or (not " + pdt + ") .Values.foo"},
		{cond: ".Values.foo"},
		{cond: `and (eq (include "helm_lib_module_ingress_enabled" .) "true") ` + pdt},
	}

	for _, tt := range tests {
		t.Run(tt.cond, func(t *testing.T) {
			pipe := conditionPipe(t, tt.cond)

			require.Equal(t, tt.whenTrue, documentationUnavailableWhen(pipe, true), "when the condition is true")
			require.Equal(t, tt.whenFalse, documentationUnavailableWhen(pipe, false), "when the condition is false")
		})
	}
}
