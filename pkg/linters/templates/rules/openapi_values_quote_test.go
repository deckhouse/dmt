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
			name: "value placed as a block via nindent is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | nindent 4 }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value placed as a block via indent is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | indent 4 }}\n",
			},
			wantCount: 0,
		},
		{
			name: "base64 value expanded via b64dec|nindent is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n{{ .Values." + valuesKey + ".foo | b64dec | nindent 2 }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value consumed by fail is never emitted, not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  {{ cat \"missing key:\" .Values." + valuesKey + ".foo | fail }}\n",
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
			name: "value embedded in a larger unquoted scalar is flagged (wrap advice)",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: prefix-{{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"wrap the whole value in quotes"},
		},
		{
			name: "value embedded in a quoted scalar is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: \"prefix-{{ .Values." + valuesKey + ".foo }}\"\n",
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
		{
			name: "value piped through squote is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | squote }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value piped through toJson is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | toJson }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value piped through toYaml is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | toYaml }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value piped through an unrelated function is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo | trim }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "only maxLength does not exempt (still flagged)",
			valuesSchema: `type: object
properties:
  foo:
    type: string
    maxLength: 10
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "empty enum does not exempt (still flagged)",
			valuesSchema: `type: object
properties:
  foo:
    type: string
    enum: []
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "empty pattern does not exempt (still flagged)",
			valuesSchema: `type: object
properties:
  foo:
    type: string
    pattern: ''
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "string branch of anyOf is flagged",
			valuesSchema: `type: object
properties:
  foo:
    anyOf:
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
			name: "deeply nested string path is flagged with full path",
			valuesSchema: `type: object
properties:
  a:
    type: object
    properties:
      b:
        type: object
        properties:
          c:
            type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  c: {{ .Values." + valuesKey + ".a.b.c }}\n",
			},
			wantCount:    1,
			wantContains: []string{"a.b.c"},
		},
		{
			name: "multiple unquoted strings in one file are all flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
  bar:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ .Values." + valuesKey + ".foo }}\n  bar: {{ .Values." + valuesKey + ".bar }}\n",
			},
			wantCount:    2,
			wantContains: []string{"foo", "bar"},
		},
		{
			name: "unquoted scalar via with is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{- with .Values." + valuesKey + ".foo }}\ndata:\n  x: {{ . }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "quoted scalar via with is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{- with .Values." + valuesKey + ".foo }}\ndata:\n  x: {{ . | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "root-scoped $.Values scalar is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ $.Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "unquoted array element via index+element range is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $i, $e := .Values." + valuesKey + ".list }}\n  - {{ $e }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element", "list"},
		},
		{
			name: "value passed to printf %s is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ printf \"%s-x\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".foo'"},
		},
		{
			name: "value passed to printf %q is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ printf \"%q\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value passed to an unknown function is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ include \"mymod.helper\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "value in a parenthesised group is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  foo: {{ (.Values." + valuesKey + ".foo) }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".foo'"},
		},
		{
			name: "risky loop variable passed to a passthrough function is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $v := .Values." + valuesKey + ".list }}\n  - {{ upper $v }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"list"},
		},
		{
			name: "risky object-subfield variable passed to printf is flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  host: {{ printf \"%s\" $s.host }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"servers[].host"},
		},
		{
			name: "quoted passthrough of a variable is not flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $v := .Values." + valuesKey + ".list }}\n  - {{ upper $v | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "index into a risky array is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  first: {{ index .Values." + valuesKey + ".list 0 }}\n",
			},
			wantCount:    1,
			wantContains: []string{"list"},
		},
		{
			name: "additionalProperties map value is not matched",
			valuesSchema: `type: object
properties:
  labels:
    type: object
    additionalProperties:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  x: {{ .Values." + valuesKey + ".labels.somekey }}\n",
			},
			wantCount: 0,
		},
		{
			name: "template in a .tpl file is scanned",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "x: {{ .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"foo"},
		},
		{
			name: "quoted array element via named range variable is not flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $x := .Values." + valuesKey + ".list }}\n  - {{ $x | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "array element via named range variable wrapped in quotes is not flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $x := .Values." + valuesKey + ".list }}\n  - \"{{ $x }}\"\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "array element via index+element range with squote is not flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $i, $x := .Values." + valuesKey + ".list }}\n  - {{ $x | squote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "unquoted named range variable inside a nested if is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $x := .Values." + valuesKey + ".list }}\n{{- if $x }}\n  - {{ $x }}\n{{- end }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element", "list"},
		},
		{
			name: "root-scoped $.Values scalar quoted inside range is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{- range .Values." + valuesKey + ".list }}\ndata:\n  x: {{ $.Values." + valuesKey + ".foo | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "unquoted string subfield of an object array element is flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  host: {{ $s.host }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element field", "servers[].host"},
		},
		{
			name: "quoted string subfield of an object array element is not flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  host: {{ $s.host | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "string subfield wrapped in quotes is not flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  host: \"{{ $s.host }}\"\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "unquoted subfield via dot-binding range is flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range .Values." + valuesKey + ".servers }}\n  host: {{ .host }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"servers[].host"},
		},
		{
			name: "unquoted deeply-nested subfield is flagged with full sub-path",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        spec:
          type: object
          properties:
            name:
              type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  name: {{ $s.spec.name }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"servers[].spec.name"},
		},
		{
			name: "subfield constrained by a pattern is not flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
          pattern: '^[a-z]+$'
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  host: {{ $s.host }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "non-string subfield of an object array element is not flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        port:
          type: integer
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  port: {{ $s.port }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "mixed object array: only unquoted risky subfields are flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
        name:
          type: string
        env:
          type: string
          enum: ["prod", "dev"]
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n" +
					"  host: {{ $s.host }}\n" + // FLAG
					"  name: {{ $s.name | quote }}\n" + // safe (quote)
					"  env: {{ $s.env }}\n" + // safe (enum)
					"{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"servers[].host"},
		},
		{
			name: "unquoted element of a string-array subfield via nested range is flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        tags:
          type: array
          items:
            type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n" +
					"{{- range $s := .Values." + valuesKey + ".servers }}\n" +
					"{{- range $t := $s.tags }}\n" +
					"  - {{ $t }}\n" +
					"{{- end }}\n" +
					"{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element from", "servers[].tags"},
		},
		{
			name: "quoted element of a string-array subfield via nested range is not flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        tags:
          type: array
          items:
            type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n" +
					"{{- range $s := .Values." + valuesKey + ".servers }}\n" +
					"{{- range $t := $s.tags }}\n" +
					"  - {{ $t | quote }}\n" +
					"{{- end }}\n" +
					"{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "nested range with dot-binding is flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        tags:
          type: array
          items:
            type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n" +
					"{{- range .Values." + valuesKey + ".servers }}\n" +
					"{{- range .tags }}\n" +
					"  - {{ . }}\n" +
					"{{- end }}\n" +
					"{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"servers[].tags"},
		},
		{
			name: "string-array subfield with item pattern via nested range is not flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        tags:
          type: array
          items:
            type: string
            pattern: '^[a-z]+$'
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n" +
					"{{- range $s := .Values." + valuesKey + ".servers }}\n" +
					"{{- range $t := $s.tags }}\n" +
					"  - {{ $t }}\n" +
					"{{- end }}\n" +
					"{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "object array subfield can be excluded by its full path",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        host:
          type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n{{- range $s := .Values." + valuesKey + ".servers }}\n  host: {{ $s.host }}\n{{- end }}\n",
			},
			excludes:  []string{"servers[].host"},
			wantCount: 0,
		},
		{
			name: "unquoted subfield of an object array nested in an object array is flagged",
			valuesSchema: `type: object
properties:
  servers:
    type: array
    items:
      type: object
      properties:
        endpoints:
          type: array
          items:
            type: object
            properties:
              url:
                type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "servers:\n" +
					"{{- range $s := .Values." + valuesKey + ".servers }}\n" +
					"{{- range $e := $s.endpoints }}\n" +
					"  url: {{ $e.url }}\n" +
					"{{- end }}\n" +
					"{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element field", "servers[].endpoints[].url"},
		},

		// --- (2) with over a single object ---
		{
			name: "unquoted subfield via with over a single object is flagged",
			valuesSchema: `type: object
properties:
  db:
    type: object
    properties:
      host:
        type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{- with .Values." + valuesKey + ".db }}\nhost: {{ .host }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".db.host'"},
		},
		{
			name: "quoted subfield via with over a single object is not flagged",
			valuesSchema: `type: object
properties:
  db:
    type: object
    properties:
      host:
        type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{- with .Values." + valuesKey + ".db }}\nhost: {{ .host | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},

		// --- (5) additionalProperties (maps) ---
		{
			name: "unquoted map value via range is flagged",
			valuesSchema: `type: object
properties:
  someMap:
    type: object
    additionalProperties:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n{{- range $k, $v := .Values." + valuesKey + ".someMap }}\n  {{ $k }}: {{ $v }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element from", "someMap"},
		},
		{
			name: "quoted map value via range is not flagged",
			valuesSchema: `type: object
properties:
  someMap:
    type: object
    additionalProperties:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n{{- range $k, $v := .Values." + valuesKey + ".someMap }}\n  {{ $k }}: {{ $v | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "map value with item pattern is not flagged",
			valuesSchema: `type: object
properties:
  someMap:
    type: object
    additionalProperties:
      type: string
      pattern: '^[a-z]+$'
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n{{- range $k, $v := .Values." + valuesKey + ".someMap }}\n  {{ $k }}: {{ $v }}\n{{- end }}\n",
			},
			wantCount: 0,
		},
		{
			name: "unquoted subfield of a map-of-objects value is flagged",
			valuesSchema: `type: object
properties:
  someMap:
    type: object
    additionalProperties:
      type: object
      properties:
        host:
          type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n{{- range $k, $s := .Values." + valuesKey + ".someMap }}\n  {{ $k }}:\n    host: {{ $s.host }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"someMap[].host"},
		},

		// --- (6) array of arrays of strings ---
		{
			name: "unquoted element of an array-of-arrays via nested range is flagged",
			valuesSchema: `type: object
properties:
  matrix:
    type: array
    items:
      type: array
      items:
        type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "rows:\n" +
					"{{- range $row := .Values." + valuesKey + ".matrix }}\n" +
					"{{- range $c := $row }}\n" +
					"  - {{ $c }}\n" +
					"{{- end }}\n" +
					"{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element from", "matrix"},
		},
		{
			name: "quoted element of an array-of-arrays is not flagged",
			valuesSchema: `type: object
properties:
  matrix:
    type: array
    items:
      type: array
      items:
        type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "rows:\n" +
					"{{- range $row := .Values." + valuesKey + ".matrix }}\n" +
					"{{- range $c := $row }}\n" +
					"  - {{ $c | quote }}\n" +
					"{{- end }}\n" +
					"{{- end }}\n",
			},
			wantCount: 0,
		},

		// --- (1) variable aliasing ---
		{
			name: "aliased scalar variable emitted unquoted is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{ $y := .Values." + valuesKey + ".foo }}\ndata:\n  x: {{ $y }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".foo'"},
		},
		{
			name: "aliased scalar variable emitted quoted is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{ $y := .Values." + valuesKey + ".foo }}\ndata:\n  x: {{ $y | quote }}\n",
			},
			wantCount: 0,
		},
		{
			name: "assignment of a safe-piped value is not tracked",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{ $y := .Values." + valuesKey + ".foo | quote }}\ndata:\n  x: {{ $y }}\n",
			},
			wantCount: 0,
		},
		{
			name: "alias of a loop element variable is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "items:\n{{- range $x := .Values." + valuesKey + ".list }}\n{{ $y := $x }}\n  - {{ $y }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element from", "list"},
		},
		{
			name: "aliased object variable subfield emitted unquoted is flagged",
			valuesSchema: `type: object
properties:
  db:
    type: object
    properties:
      host:
        type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{ $y := .Values." + valuesKey + ".db }}\ndata:\n  host: {{ $y.host }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".db.host'"},
		},
		{
			name: "with over an object variable is flagged",
			valuesSchema: `type: object
properties:
  db:
    type: object
    properties:
      host:
        type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{ $s := .Values." + valuesKey + ".db }}\n{{- with $s }}\nhost: {{ .host }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".db.host'"},
		},
		{
			name: "array assigned to a variable then ranged is flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{ $x := .Values." + valuesKey + ".list }}\nitems:\n{{- range $x }}\n  - {{ . }}\n{{- end }}\n",
			},
			wantCount:    1,
			wantContains: []string{"array element from", "list"},
		},
		{
			name: "array assigned to a variable then ranged with quote is not flagged",
			valuesSchema: `type: object
properties:
  list:
    type: array
    items:
      type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "{{ $x := .Values." + valuesKey + ".list }}\nitems:\n{{- range $x }}\n  - {{ . | quote }}\n{{- end }}\n",
			},
			wantCount: 0,
		},

		// --- (4) cross-template flow ---
		{
			name: "risky scalar passed to a define that emits it unquoted is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"mymod.raw\" -}}\n{{ . }}\n{{- end -}}\n",
				"templates/cm.yaml":      "data:\n  x: {{ include \"mymod.raw\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".foo'", "mymod.raw"},
		},
		{
			name: "risky object subfield emitted unquoted by a define is flagged",
			valuesSchema: `type: object
properties:
  db:
    type: object
    properties:
      host:
        type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"mymod.conn\" -}}\nhost: {{ .host }}\n{{- end -}}\n",
				"templates/cm.yaml":      "{{ include \"mymod.conn\" .Values." + valuesKey + ".db }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".db.host'", "mymod.conn"},
		},
		{
			name: "quoted emission in a define is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"mymod.raw\" -}}\n{{ . | quote }}\n{{- end -}}\n",
				"templates/cm.yaml":      "data:\n  x: {{ include \"mymod.raw\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "include output quoted as a whole is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"mymod.raw\" -}}\n{{ . }}\n{{- end -}}\n",
				"templates/cm.yaml":      "data:\n  x: {{ include \"mymod.raw\" .Values." + valuesKey + ".foo | quote }}\n",
			},
			wantCount: 0,
		},
		{
			name: "include of an unknown external template is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/cm.yaml": "data:\n  x: {{ include \"helm_lib.foo\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "transitive define forwarding is flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"a\" -}}\n{{ include \"b\" . }}\n{{- end -}}\n" +
					"{{- define \"b\" -}}\n{{ . }}\n{{- end -}}\n",
				"templates/cm.yaml": "data:\n  x: {{ include \"a\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount:    1,
			wantContains: []string{"value '.Values." + valuesKey + ".foo'"},
		},
		{
			name: "constrained scalar passed to an emitting define is not flagged",
			valuesSchema: `type: object
properties:
  foo:
    type: string
    pattern: '^[a-z]+$'
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"mymod.raw\" -}}\n{{ . }}\n{{- end -}}\n",
				"templates/cm.yaml":      "data:\n  x: {{ include \"mymod.raw\" .Values." + valuesKey + ".foo }}\n",
			},
			wantCount: 0,
		},
		{
			name: "array subfield ranged and emitted unquoted by a define is flagged",
			valuesSchema: `type: object
properties:
  config:
    type: object
    properties:
      items:
        type: array
        items:
          type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"mymod.args\" -}}\n{{- range .items }}\n- {{ . }}\n{{- end }}\n{{- end -}}\n",
				"templates/cm.yaml":      "args:\n{{ include \"mymod.args\" .Values." + valuesKey + ".config }}\n",
			},
			wantCount:    1,
			wantContains: []string{"config.items", "mymod.args"},
		},
		{
			name: "array subfield ranged and quoted by a define is not flagged",
			valuesSchema: `type: object
properties:
  config:
    type: object
    properties:
      items:
        type: array
        items:
          type: string
`,
			files: map[string]string{
				"templates/_helpers.tpl": "{{- define \"mymod.args\" -}}\n{{- range .items }}\n- {{ . | quote }}\n{{- end }}\n{{- end -}}\n",
				"templates/cm.yaml":      "args:\n{{ include \"mymod.args\" .Values." + valuesKey + ".config }}\n",
			},
			wantCount: 0,
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
