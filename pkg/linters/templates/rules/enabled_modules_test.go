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
	"os"
	"path/filepath"
	"testing"

	"github.com/gojuno/minimock/v3"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestEnabledModulesRule_CheckEnabledModules(t *testing.T) {
	tests := []struct {
		name           string
		templateFiles  map[string]string
		expectedErrors []string
		expectedLines  []int
	}{
		{
			name: "should detect single enabledModules usage in template file",
			templateFiles: map[string]string{
				"templates/deployment.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  template:
    spec:
      containers:
      - name: test
        image: test:latest
{{- if .Values.global.enabledModules | has "cni-cilium" }}
        env:
        - name: CILIUM_ENABLED
          value: "true"
{{- end }}
`,
			},
			expectedErrors: []string{
				`Found usage of .Values.global.enabledModules | has "cni-cilium"`,
			},
			expectedLines: []int{12},
		},
		{
			name: "should detect multiple enabledModules usages in one file",
			templateFiles: map[string]string{
				"templates/configmap.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key1: "value1"
{{ if .Values.global.enabledModules | has "module-a" }}
  module-a: "enabled"
{{ end }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: another-config
data:
{{- if .Values.global.enabledModules | has "module-b" }}
  module-b: "enabled"
{{- end }}
`,
			},
			expectedErrors: []string{
				`Found usage of .Values.global.enabledModules | has "module-a"`,
				`Found usage of .Values.global.enabledModules | has "module-b"`,
			},
			expectedLines: []int{8, 17},
		},
		{
			name: "should handle template with parentheses and whitespace trimming",
			templateFiles: map[string]string{
				"templates/deployment.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  template:
    spec:
{{- if ( .Values.global.enabledModules | has "some-module" ) }}
      containers:
      - name: test
        image: test:latest
{{- end }}
`,
			},
			expectedErrors: []string{
				`Found usage of .Values.global.enabledModules | has "some-module"`,
			},
			expectedLines: []int{9},
		},
		{
			name: "should not report error when pattern is not present",
			templateFiles: map[string]string{
				"templates/deployment.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  template:
    spec:
      containers:
      - name: test
        image: test:latest
{{- if .Capabilities.APIVersions.Has "deckhouse.io/v1alpha1/ModuleConfig" }}
        env:
        - name: MODULE_ENABLED
          value: "true"
{{- end }}
`,
			},
			expectedErrors: []string{},
			expectedLines:  []int{},
		},
		{
			name: "should detect with extra whitespace in has expression",
			templateFiles: map[string]string{
				"templates/deployment.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
{{- if .Values.global.enabledModules|has "my-module" }}
  replicas: 3
{{- end }}
`,
			},
			expectedErrors: []string{
				`Found usage of .Values.global.enabledModules | has "my-module"`,
			},
			expectedLines: []int{7},
		},
		{
			name: "should match in .tpl files",
			templateFiles: map[string]string{
				"templates/_helpers.tpl": `
{{- define "check-module" -}}
{{- if .Values.global.enabledModules | has "my-module" -}}
  enabled
{{- else -}}
  disabled
{{- end -}}
{{- end -}}
`,
			},
			expectedErrors: []string{
				`Found usage of .Values.global.enabledModules | has "my-module"`,
			},
			expectedLines: []int{3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "enabled-modules-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Create module structure
			modulePath := filepath.Join(tempDir, "module")
			if err := os.MkdirAll(modulePath, 0755); err != nil {
				t.Fatalf("Failed to create module dir: %v", err)
			}

			// Create template files
			for filePath, content := range tt.templateFiles {
				fullPath := filepath.Join(modulePath, filePath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatalf("Failed to create template dir: %v", err)
				}

				if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
					t.Fatalf("Failed to write template file: %v", err)
				}
			}

			// Create mock module
			mc := minimock.NewController(t)

			mockModule := mocks.NewModuleMock(mc)
			mockModule.GetPathMock.Return(modulePath)

			// Create error list
			errorList := errors.NewLintRuleErrorsList()

			// Create rule
			rule := NewEnabledModulesRule()

			// Run validation
			rule.CheckEnabledModules(mockModule, errorList)

			// Check results
			errs := errorList.GetErrors()
			if len(errs) != len(tt.expectedErrors) {
				t.Errorf("Expected %d errors, got %d", len(tt.expectedErrors), len(errs))
			}

			for i, expectedError := range tt.expectedErrors {
				if i >= len(errs) {
					t.Errorf("Expected error at index %d: %s", i, expectedError)
					continue
				}

				if !containsStr(errs[i].Text, expectedError) {
					t.Errorf("Expected error to contain '%s', got: %s", expectedError, errs[i].Text)
				}
			}

			for i, expectedLine := range tt.expectedLines {
				if i >= len(errs) {
					t.Errorf("Expected line at index %d: %d", i, expectedLine)
					continue
				}

				if errs[i].LineNumber != expectedLine {
					t.Errorf("Expected line %d, got %d for error: %s", expectedLine, errs[i].LineNumber, errs[i].Text)
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
