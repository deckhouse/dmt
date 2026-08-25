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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestDeckhouseValidationsRule_Run(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantCount    int
		wantContains []string
	}{
		{
			name: "valid top-level rule compiles",
			content: `
type: object
properties:
  replicas:
    type: integer
x-deckhouse-validations:
  - expression: "self.replicas >= 1"
    message: "replicas must be >= 1"
`,
			wantCount: 0,
		},
		{
			name: "valid transition rule with oldSelf compiles",
			content: `
type: object
properties:
  clusterDomain:
    type: string
x-deckhouse-validations:
  - expression: "self.clusterDomain == oldSelf.clusterDomain"
    message: "clusterDomain is immutable"
`,
			wantCount: 0,
		},
		{
			name: "no validations extension at all",
			content: `
type: object
properties:
  foo:
    type: string
`,
			wantCount: 0,
		},
		{
			name: "not a list",
			content: `
type: object
x-deckhouse-validations: "not a list"
`,
			wantCount:    1,
			wantContains: []string{"must be a list", "got a string"},
		},
		{
			name: "empty list",
			content: `
type: object
x-deckhouse-validations: []
`,
			wantCount:    1,
			wantContains: []string{"at least one"},
		},
		{
			name: "entry is not a mapping",
			content: `
type: object
x-deckhouse-validations:
  - "just a string"
`,
			wantCount:    1,
			wantContains: []string{"non-empty mapping"},
		},
		{
			name: "entry missing expression",
			content: `
type: object
x-deckhouse-validations:
  - message: "no expression here"
`,
			wantCount:    1,
			wantContains: []string{`"expression"`},
		},
		{
			name: "entry with empty expression",
			content: `
type: object
x-deckhouse-validations:
  - expression: ""
    message: "empty expression"
`,
			wantCount:    1,
			wantContains: []string{`"expression"`},
		},
		{
			name: "entry missing message",
			content: `
type: object
x-deckhouse-validations:
  - expression: "self.replicas >= 1"
`,
			wantCount:    1,
			wantContains: []string{`"message"`},
		},
		{
			name: "expression does not compile (undeclared reference)",
			content: `
type: object
x-deckhouse-validations:
  - expression: "foo.bar >= 1"
    message: "bad expression"
`,
			wantCount:    1,
			wantContains: []string{"not a valid CEL expression"},
		},
		{
			name: "expression has a syntax error",
			content: `
type: object
x-deckhouse-validations:
  - expression: "self.replicas >"
    message: "syntax error"
`,
			wantCount:    1,
			wantContains: []string{"not a valid CEL expression"},
		},
		{
			name: "nested validations report their path",
			content: `
type: object
properties:
  registry:
    type: object
    x-deckhouse-validations:
      - expression: "nope("
        message: "broken"
`,
			wantCount:    1,
			wantContains: []string{"properties.registry.x-deckhouse-validations[0]", "not a valid CEL expression"},
		},
		{
			name: "x-kubernetes-validations is flagged as ignored here",
			content: `
type: object
x-kubernetes-validations:
  - rule: "self.replicas >= 1"
    message: "min replicas"
`,
			wantCount:    1,
			wantContains: []string{"x-kubernetes-validations", "x-deckhouse-validations"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, "config-values.yaml")
			require.NoError(t, os.WriteFile(filePath, []byte(tt.content), 0o600))

			errorList := errors.NewLintRuleErrorsList()
			rule := NewDeckhouseValidationsRule(&pkg.OpenAPILinterConfig{}, moduleAt(t, dir), errorList)

			rule.checkFile(filePath)

			errs := errorList.GetErrors()
			require.Len(t, errs, tt.wantCount)

			for _, e := range errs {
				// Every finding of this rule is currently emitted at warn (explicit
				// .Warnf) during the migration period.
				assert.Equal(t, pkg.Warn, e.Level, "finding: %s", e.Text)
				assert.Equal(t, "config-values.yaml", e.FilePath)
			}

			for _, want := range tt.wantContains {
				found := false

				for i := range errs {
					if strings.Contains(errs[i].Text, want) {
						found = true

						break
					}
				}

				assert.True(t, found, "expected a finding containing %q, got %v", want, texts(errs))
			}
		})
	}
}

// TestDeckhouseValidationsRule_MultipleFindingsPerEntry checks that an entry
// missing both fields yields one finding per missing field.
func TestDeckhouseValidationsRule_MultipleFindingsPerEntry(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "config-values.yaml")
	content := `
type: object
x-deckhouse-validations:
  - foo: bar
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))

	errorList := errors.NewLintRuleErrorsList()
	rule := NewDeckhouseValidationsRule(&pkg.OpenAPILinterConfig{}, moduleAt(t, dir), errorList)

	rule.checkFile(filePath)

	errs := errorList.GetErrors()
	// A non-empty mapping without expression and without message: two findings.
	require.Len(t, errs, 2)
	assert.Equal(t, pkg.Warn, errs[0].Level)
	assert.Equal(t, pkg.Warn, errs[1].Level)
}

// TestDeckhouseValidationsRule_BrokenYAMLIsSilent verifies the rule does not
// re-report a YAML syntax error (the enum/ha rules already surface it).
func TestDeckhouseValidationsRule_BrokenYAMLIsSilent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "config-values.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte("type: object\n\tbad: indent\n"), 0o600))

	errorList := errors.NewLintRuleErrorsList()
	rule := NewDeckhouseValidationsRule(&pkg.OpenAPILinterConfig{}, moduleAt(t, dir), errorList)

	rule.checkFile(filePath)

	assert.Empty(t, errorList.GetErrors())
}

func texts(errs []pkg.LinterError) []string {
	out := make([]string, 0, len(errs))
	for i := range errs {
		out = append(out, errs[i].Text)
	}

	return out
}
