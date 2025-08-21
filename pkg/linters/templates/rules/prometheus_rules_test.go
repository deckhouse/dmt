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

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMe(_ *testing.T) {
	aaa := `
{{ $namespace := "d8-monitoring" }}

{{- include "helm_lib_prometheus_rules_recursion" (list . $namespace "monitoring/prometheus-rules/extended-monitoring") }}
{{- if .Values.extendedMonitoring.imageAvailability.exporterEnabled }}
  {{- include "helm_lib_prometheus_rules_recursion" (list . $namespace "monitoring/prometheus-rules/image-availability") }}
{{- end }}
{{- if .Values.extendedMonitoring.certificates.exporterEnabled }}
  {{- include "helm_lib_prometheus_rules_recursion" (list . $namespace "monitoring/prometheus-rules/certificates") }}
{{- end }}
`

	isContentMatching([]byte(aaa), "include \"helm_lib_prometheus_rules")
}

func TestMarshalStorageObject(t *testing.T) {
	tests := []struct {
		name           string
		input          storage.StoreObject
		expectedYAML   string
		expectingError bool
	}{
		{
			name: "Valid input with simple spec",
			input: storage.StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"spec": map[string]any{
							"key":   "value",
							"count": 42,
						},
					},
				},
			},
			expectedYAML: `count: 42
key: value
`,
			expectingError: false,
		},
		{
			name: "Empty spec",
			input: storage.StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"spec": map[string]any{},
					},
				},
			},
			expectedYAML:   "{}\n",
			expectingError: false,
		},
		{
			name: "Missing spec field",
			input: storage.StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: map[string]any{},
				},
			},
			expectedYAML:   "",
			expectingError: true,
		},
		{
			name: "Nested spec structure",
			input: storage.StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"spec": map[string]any{
							"nested": map[string]any{
								"key": "value",
							},
						},
					},
				},
			},
			expectedYAML: `nested:
  key: value
`,
			expectingError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalStorageObject(tt.input)

			if tt.expectingError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedYAML, string(result))
			}
		})
	}
}

func TestValidatePrometheusRules(t *testing.T) {
	tests := []struct {
		name           string
		modulePath     string
		files          map[string]string
		expectedErrors []string
	}{
		{
			name:       "Missing templates/monitoring.yaml file",
			modulePath: "/module",
			files: map[string]string{
				"/module/monitoring/prometheus-rules/rule.yaml": "some content",
			},
			expectedErrors: []string{
				"Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file",
			},
		},
		{
			name:       "Missing monitoring/prometheus-rules folder",
			modulePath: "/module",
			files: map[string]string{
				"/module/templates/monitoring.yaml": "some content",
			},
			expectedErrors: nil,
		},
		{
			name:       "Invalid content in templates/monitoring.yaml",
			modulePath: "/module",
			files: map[string]string{
				"/module/templates/monitoring.yaml":             "invalid content",
				"/module/monitoring/prometheus-rules/rule.yaml": "some content",
			},
			expectedErrors: []string{
				"The content of the 'templates/monitoring.yaml' should be equal to:",
			},
		},
		{
			name:       "Valid content in templates/monitoring.yaml",
			modulePath: "/module",
			files: map[string]string{
				"/module/templates/monitoring.yaml":             `{{- include "helm_lib_prometheus_rules" (list . "d8-monitoring") }}`,
				"/module/monitoring/prometheus-rules/rule.yaml": "some content",
			},
			expectedErrors: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock file system
			tmpDir := t.TempDir()
			modulePath := filepath.Join(tmpDir, tt.modulePath)
			for path, content := range tt.files {
				filePath := filepath.Join(tmpDir, path)
				err := os.MkdirAll(filepath.Dir(filePath), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte(content), 0600)
				require.NoError(t, err)
			}
			defer func() {
				_ = os.RemoveAll(modulePath)
			}()

			// Mock module
			mockModuleProm := &mockModuleProm{path: modulePath}

			// Run validation
			rule := NewPrometheusRule(nil)
			errorList := errors.NewLintRuleErrorsList()
			rule.ValidatePrometheusRules(mockModuleProm, errorList)

			// Assert errors
			if len(tt.expectedErrors) == 0 {
				assert.Empty(t, errorList.GetErrors())
			} else {
				assert.Len(t, errorList.GetErrors(), len(tt.expectedErrors))
				for i, expectedError := range tt.expectedErrors {
					assert.Contains(t, errorList.GetErrors()[i].Text, expectedError)
				}
			}
		})
	}
}

type mockModuleProm struct {
	path string
}

func (m *mockModuleProm) GetPath() string {
	return m.path
}
