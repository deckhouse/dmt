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

func TestClusterDomainRule_ValidateClusterDomainInTemplates(t *testing.T) {
	tests := []struct {
		name           string
		modulePath     string
		templateFiles  map[string]string
		expectedErrors []string
	}{
		{
			name:       "should detect cluster.local in template file",
			modulePath: "/test/module",
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
        env:
        - name: CLUSTER_DOMAIN
          value: "cluster.local"
`,
			},
			expectedErrors: []string{
				"File contains hardcoded 'cluster.local' substring. Use '.Values.global.clusterConfiguration.clusterDomain' instead for dynamic cluster domain configuration.",
			},
		},
		{
			name:       "should not detect cluster.local in non-template file",
			modulePath: "/test/module",
			templateFiles: map[string]string{
				"templates/README.md": `
This file contains cluster.local but it's not a template file.
`,
			},
			expectedErrors: []string{},
		},
		{
			name:       "should not detect cluster.local when not present",
			modulePath: "/test/module",
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
        env:
        - name: CLUSTER_DOMAIN
          value: "example.com"
`,
			},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "cluster-domain-test")
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
			rule := NewClusterDomainRule()

			// Run validation
			rule.ValidateClusterDomainInTemplates(mockModule, errorList)

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
				if !contains(errs[i].Text, expectedError) {
					t.Errorf("Expected error to contain '%s', got: %s", expectedError, errs[i].Text)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
