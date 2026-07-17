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
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestCertificateGatewayIssuerRule_ValidateCertificateGatewayIssuer(t *testing.T) {
	tests := []struct {
		name           string
		templateFiles  map[string]string
		storageObjects map[storage.ResourceIndex]storage.StoreObject
		excludeRules   []pkg.KindRuleExclude
		expectedErrors []string
	}{
		{
			name: "should detect forbidden printf issuer in Certificate",
			templateFiles: map[string]string{
				"templates/certificate.yaml": `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
    kind: ClusterIssuer
`,
			},
			storageObjects: map[storage.ResourceIndex]storage.StoreObject{
				{
					Kind: "Certificate",
					Name: "gateway-cert",
				}: {
					AbsPath: "templates/certificate.yaml",
					Unstructured: unstructured.Unstructured{Object: map[string]any{
						"kind": "Certificate",
						"metadata": map[string]any{
							"name": "gateway-cert",
						},
					}},
				},
			},
			expectedErrors: []string{certificateGatewayIssuerMessage},
		},
		{
			name: "should accept recommended include helper",
			templateFiles: map[string]string{
				"templates/certificate.yaml": `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name_for_gateway_api" . }}
    kind: ClusterIssuer
`,
			},
			storageObjects: map[storage.ResourceIndex]storage.StoreObject{
				{
					Kind: "Certificate",
					Name: "gateway-cert",
				}: {
					AbsPath: "templates/certificate.yaml",
					Unstructured: unstructured.Unstructured{Object: map[string]any{
						"kind": "Certificate",
						"metadata": map[string]any{
							"name": "gateway-cert",
						},
					}},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "should ignore printf outside Certificate objects",
			templateFiles: map[string]string{
				"templates/configmap.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  issuer: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
`,
			},
			storageObjects: map[storage.ResourceIndex]storage.StoreObject{},
			expectedErrors: []string{},
		},
		{
			name: "should skip excluded certificate",
			templateFiles: map[string]string{
				"templates/certificate.yaml": `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
    kind: ClusterIssuer
`,
			},
			storageObjects: map[storage.ResourceIndex]storage.StoreObject{
				{
					Kind: "Certificate",
					Name: "gateway-cert",
				}: {
					AbsPath: "templates/certificate.yaml",
					Unstructured: unstructured.Unstructured{Object: map[string]any{
						"kind": "Certificate",
						"metadata": map[string]any{
							"name": "gateway-cert",
						},
					}},
				},
			},
			excludeRules: []pkg.KindRuleExclude{
				{
					Kind: "Certificate",
					Name: "gateway-cert",
				},
			},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "certificate-gateway-issuer-test")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			modulePath := filepath.Join(tempDir, "module")
			require.NoError(t, os.MkdirAll(modulePath, 0o755))

			for filePath, content := range tt.templateFiles {
				fullPath := filepath.Join(modulePath, filePath)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
			}

			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)
			objects := make(map[storage.ResourceIndex]storage.StoreObject, len(tt.storageObjects))
			for index, object := range tt.storageObjects {
				object.AbsPath = filepath.Join(modulePath, object.AbsPath)
				objects[index] = object
			}
			mockModule.GetStorageMock.Return(objects)

			errorList := errors.NewLintRuleErrorsList()
			rule := NewCertificateGatewayIssuerRule(tt.excludeRules)

			rule.ValidateCertificateGatewayIssuer(mockModule, errorList)

			errs := errorList.GetErrors()
			require.Len(t, errs, len(tt.expectedErrors))

			for i, expectedError := range tt.expectedErrors {
				assert.Contains(t, errs[i].Text, expectedError)
			}
		})
	}
}

func TestContainsForbiddenGatewayIssuer(t *testing.T) {
	assert.True(t, containsForbiddenGatewayIssuer(strings.TrimSpace(`
kind: Certificate
spec:
  issuerRef:
    name: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
`)))
	assert.False(t, containsForbiddenGatewayIssuer(strings.TrimSpace(`
kind: Certificate
spec:
  issuerRef:
    name: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name_for_gateway_api" . }}
`)))
}
