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
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	// moduleYAMLInScope targets a Deckhouse version at/above the "-crd" removal, so
	// the rule is active.
	moduleYAMLInScope = "name: test-module\nrequirements:\n  deckhouse: \">= 1.65\"\n"
	// moduleYAMLBelowThreshold still supports a Deckhouse release older than the
	// removal, so the rule must stay silent.
	moduleYAMLBelowThreshold = "name: test-module\nrequirements:\n  deckhouse: \">= 1.64\"\n"
	// moduleYAMLNoRequirement declares no deckhouse requirement — out of scope.
	moduleYAMLNoRequirement = "name: test-module\n"
)

// writeCRDModule builds a temporary module directory containing an optional
// module.yaml (skipped when moduleYAML is empty) and the given template files
// (keyed by path relative to the module root). It returns the module path.
func writeCRDModule(t *testing.T, moduleYAML string, templateFiles map[string]string) string {
	t.Helper()

	modulePath := filepath.Join(t.TempDir(), "module")
	require.NoError(t, os.MkdirAll(modulePath, 0o755))

	if moduleYAML != "" {
		require.NoError(t, os.WriteFile(filepath.Join(modulePath, moduleConfigFilename), []byte(moduleYAML), 0o600))
	}

	for relPath, content := range templateFiles {
		fullPath := filepath.Join(modulePath, relPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
	}

	return modulePath
}

func crdMockModule(t *testing.T, modulePath string) *mocks.ModuleMock {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(modulePath)

	return m
}

func TestCRDEnabledModulesRule_CheckCRDEnabledModules(t *testing.T) {
	tests := []struct {
		name          string
		moduleYAML    string
		templateFiles map[string]string
		wantCount     int
		wantContains  []string
		wantLines     []int
	}{
		{
			name:       "flags a -crd reference for a module targeting the removal version",
			moduleYAML: moduleYAMLInScope,
			templateFiles: map[string]string{
				"templates/deployment.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: x
{{- if .Values.global.enabledModules | has "operator-prometheus-crd" }}
data: {}
{{- end }}
`,
			},
			wantCount:    1,
			wantContains: []string{`"operator-prometheus-crd"`, `use "operator-prometheus"`, "1.65.0"},
			wantLines:    []int{5},
		},
		{
			name:       "flags multiple -crd references in one file",
			moduleYAML: moduleYAMLInScope,
			templateFiles: map[string]string{
				"templates/configmap.yaml": `{{- if .Values.global.enabledModules | has "prometheus-crd" }}
a: b
{{- end }}
{{- if .Values.global.enabledModules | has "user-authn-crd" }}
c: d
{{- end }}
`,
			},
			wantCount:    2,
			wantContains: []string{`use "prometheus"`, `use "user-authn"`},
			wantLines:    []int{1, 4},
		},
		{
			name:       "ignores a reference to a module without the -crd suffix",
			moduleYAML: moduleYAMLInScope,
			templateFiles: map[string]string{
				"templates/deployment.yaml": `{{- if .Values.global.enabledModules | has "cni-cilium" }}
x: y
{{- end }}
`,
			},
			wantCount: 0,
		},
		{
			name:       "flags a -crd reference inside a .tpl file",
			moduleYAML: moduleYAMLInScope,
			templateFiles: map[string]string{
				"templates/_helpers.tpl": `{{- define "check" -}}
{{- if .Values.global.enabledModules | has "vertical-pod-autoscaler-crd" -}}
on
{{- end -}}
{{- end -}}
`,
			},
			wantCount:    1,
			wantContains: []string{`use "vertical-pod-autoscaler"`},
			wantLines:    []int{2},
		},
		{
			name:       "flags only the -crd reference when mixed with a plain one",
			moduleYAML: moduleYAMLInScope,
			templateFiles: map[string]string{
				"templates/deployment.yaml": `{{- if .Values.global.enabledModules | has "cni-cilium" }}
a: b
{{- end }}
{{- if .Values.global.enabledModules | has "snapshot-controller-crd" }}
c: d
{{- end }}
`,
			},
			wantCount:    1,
			wantContains: []string{`use "snapshot-controller"`},
			wantLines:    []int{4},
		},
		{
			name:       "skips a module that still supports a Deckhouse release below the threshold",
			moduleYAML: moduleYAMLBelowThreshold,
			templateFiles: map[string]string{
				"templates/deployment.yaml": `{{- if .Values.global.enabledModules | has "operator-prometheus-crd" }}
x: y
{{- end }}
`,
			},
			wantCount: 0,
		},
		{
			name:       "skips a module without a deckhouse requirement",
			moduleYAML: moduleYAMLNoRequirement,
			templateFiles: map[string]string{
				"templates/deployment.yaml": `{{- if .Values.global.enabledModules | has "operator-prometheus-crd" }}
x: y
{{- end }}
`,
			},
			wantCount: 0,
		},
		{
			name:       "skips a module without module.yaml",
			moduleYAML: "",
			templateFiles: map[string]string{
				"templates/deployment.yaml": `{{- if .Values.global.enabledModules | has "operator-prometheus-crd" }}
x: y
{{- end }}
`,
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := writeCRDModule(t, tt.moduleYAML, tt.templateFiles)

			errorList := errors.NewLintRuleErrorsList()
			NewCRDEnabledModulesRule(nil, nil).CheckCRDEnabledModules(crdMockModule(t, modulePath), errorList)

			errs := errorList.GetErrors()
			require.Len(t, errs, tt.wantCount)

			for _, want := range tt.wantContains {
				found := false

				for i := range errs {
					if containsStr(errs[i].Text, want) {
						found = true
						break
					}
				}

				assert.Truef(t, found, "expected a finding containing %q, got %+v", want, errs)
			}

			for i, wantLine := range tt.wantLines {
				if i < len(errs) {
					assert.Equalf(t, wantLine, errs[i].LineNumber, "unexpected line for finding %d: %s", i, errs[i].Text)
				}
			}
		})
	}
}

func TestCRDEnabledModulesRule_Excludes(t *testing.T) {
	files := map[string]string{
		"templates/skip-me.yaml": `{{- if .Values.global.enabledModules | has "operator-prometheus-crd" }}
x: y
{{- end }}
`,
	}
	modulePath := writeCRDModule(t, moduleYAMLInScope, files)

	errorList := errors.NewLintRuleErrorsList()
	NewCRDEnabledModulesRule([]pkg.StringRuleExclude{"templates/skip-me.yaml"}, nil).
		CheckCRDEnabledModules(crdMockModule(t, modulePath), errorList)

	assert.Empty(t, errorList.GetErrors())
}

func TestCRDEnabledModulesRule_Autofix(t *testing.T) {
	const templateName = "templates/deployment.yaml"

	original := `apiVersion: v1
kind: ConfigMap
metadata:
  name: x
{{- if .Values.global.enabledModules | has "operator-prometheus-crd" }}
prometheus: "on"
{{- end }}
{{- if .Values.global.enabledModules | has "cni-cilium" }}
cilium: "on"
{{- end }}
{{- if .Values.global.enabledModules | has "user-authn-crd" }}
authn: "on"
{{- end }}
`

	modulePath := writeCRDModule(t, moduleYAMLInScope, map[string]string{templateName: original})

	errorList := errors.NewLintRuleErrorsList()
	NewCRDEnabledModulesRule(nil, nil).CheckCRDEnabledModules(crdMockModule(t, modulePath), errorList)

	fixes := errorList.GetFixes()
	require.Len(t, fixes, 2, "each -crd finding should carry a fix")

	for _, fix := range fixes {
		fix()
	}

	// After applying the fixes the fixed findings are dropped from the error list.
	assert.Empty(t, errorList.GetErrors())

	fixed, err := os.ReadFile(filepath.Join(modulePath, templateName))
	require.NoError(t, err)

	got := string(fixed)

	assert.Contains(t, got, `has "operator-prometheus"`)
	assert.NotContains(t, got, `operator-prometheus-crd`)
	assert.Contains(t, got, `has "user-authn"`)
	assert.NotContains(t, got, `user-authn-crd`)
	// Plain (non -crd) references must be left untouched.
	assert.Contains(t, got, `has "cni-cilium"`)

	// Re-running the rule on the fixed module must find nothing (idempotency).
	rerun := errors.NewLintRuleErrorsList()
	NewCRDEnabledModulesRule(nil, nil).CheckCRDEnabledModules(crdMockModule(t, modulePath), rerun)
	assert.Empty(t, rerun.GetErrors())
}
