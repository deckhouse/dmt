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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/modules/values"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const testModuleName = "test-mod"

// configValuesObject is a minimal config-values.yaml that adds no properties of its own.
const configValuesObject = "type: object\nproperties: {}\n"

func TestOpenAPIValuesQuoteRule(t *testing.T) {
	valuesKey := values.ModuleCamelName(testModuleName) // "testMod"

	tests := []struct {
		name          string
		valuesSchema  string
		files         map[string]string
		excludes      []string
		wantCount     int
		wantContains  []string
		wantNotExists string
	}{
		{
			name: "unquoted scalar string is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo", "must be quoted"},
		},
		{
			name: "value piped through quote is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | quote }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value wrapped in double quotes is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: \"{{ .Values." + valuesKey + ".foo }}\"\n",
			},
			wantCount: 0,
		},
		{
			name: "value wrapped in single quotes is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: '{{ .Values." + valuesKey + ".foo }}'\n",
			},
			wantCount: 0,
		},
		{
			name: "string with pattern is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
    pattern: '^[a-z]+$'
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "string with enum is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
    enum: ["A", "B"]
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "string with format is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
    format: date-time
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "integer is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: integer
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "nested string path is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: object
    properties:
      bar:
        type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  bar: {{ .Values." + valuesKey + ".foo.bar }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo.bar"},
		},
		{
			name: "nullable string is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: ["string", "null"]
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "string via $ref is flagged",
			valuesSchema: `type: object
properties:
  foo:
    $ref: '#/definitions/nameType'
definitions:
  nameType:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "string via allOf is flagged",
			valuesSchema: `type: object
properties:
  foo:
    allOf:
      - type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "string branch of oneOf is flagged",
			valuesSchema: `type: object
properties:
  foo:
    oneOf:
      - type: string
      - type: integer
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "unquoted array element via range is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range .Values." + valuesKey + ".list }}\n  - {{ . }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element", "list"},
		},
		{
			name: "quoted array element via range is not flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range .Values." + valuesKey + ".list }}\n  - {{ . | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "unquoted array element via named range variable is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $x := .Values." + valuesKey + ".list }}\n  - {{ $x }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element", "list"},
		},
		{
			name: "array of strings with pattern is not flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
      pattern: '^[a-z]+$'
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range .Values." + valuesKey + ".list }}\n  - {{ . }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value embedded in a larger scalar is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: prefix-{{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value inside a block scalar is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  script: |\n    echo {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value used in an if condition is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{- if .Values." + valuesKey + ".foo }}\ndata: {}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "excluded value path is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			excludes:  []string{"foo"},
			wantCount: 0,
		},
		{
			name: "value piped through b64enc is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | b64enc }}\n",
			},
			wantCount: 0,
		},
		{
			name: "unquoted value with default is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + `.foo | default "x" }}` + "\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := t.TempDir()

			writeFile(t, modulePath, "openapi/values.yaml", tt.valuesSchema)
			writeFile(t, modulePath, "openapi/config-values.yaml", configValuesObject)

			for rel, content := range tt.files {
				writeFile(t, modulePath, rel, content)
			}

			mc := minimock.NewController(t)
			module := mocks.NewModuleMock(mc)
			module.GetPathMock.Return(modulePath)
			module.GetNameMock.Return(testModuleName)

			errorList := errors.NewLintRuleErrorsList()

			excludes := make([]pkg.StringRuleExclude, 0, len(tt.excludes))
			for _, e := range tt.excludes {
				excludes = append(excludes, pkg.StringRuleExclude(e))
			}

			NewOpenAPIValuesQuoteRule(excludes).CheckStringValuesQuoted(module, errorList)

			errs := errorList.GetErrors()
			if len(errs) != tt.wantCount {
				t.Fatalf("expected %d finding(s), got %d: %s", tt.wantCount, len(errs), formatErrs(errs))
			}

			for _, want := range tt.wantContains {
				if !anyContains(errs, want) {
					t.Errorf("expected a finding containing %q, got: %s", want, formatErrs(errs))
				}
			}
		})
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()

	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}

	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func anyContains(errs []pkg.LinterError, substr string) bool {
	for i := range errs {
		if strings.Contains(errs[i].Text, substr) {
			return true
		}
	}

	return false
}

func formatErrs(errs []pkg.LinterError) string {
	var b strings.Builder
	for i := range errs {
		b.WriteString("\n  - ")
		b.WriteString(errs[i].Text)
	}

	if b.Len() == 0 {
		return "(none)"
	}

	return b.String()
}
