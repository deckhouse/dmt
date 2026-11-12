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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestFindHardcodedLetsencryptIssuers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []issuerFinding
	}{
		{
			name: "printf letsencrypt-gateway issuer",
			content: `{{- $moduleGateway := dict "name" "main" }}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
    kind: ClusterIssuer
`,
			expected: []issuerFinding{{
				certificateName: "gateway-cert",
				line:            8,
				value:           `{{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}`,
			}},
		},
		{
			name: "hardcoded issuer under a helm conditional is still detected",
			content: `{{- $moduleGateway := dict }}
{{- include "helm_lib_module_gateway" (list . $moduleGateway) }}
{{- if and $moduleGateway (eq (include "helm_lib_module_https_mode" .) "CertManager") }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
    kind: ClusterIssuer
{{- end }}
`,
			expected: []issuerFinding{{
				certificateName: "gateway-cert",
				line:            11,
				value:           `{{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}`,
			}},
		},
		{
			name: "plain letsencrypt issuer",
			content: `apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ingress-cert
spec:
  issuerRef:
    name: letsencrypt
    kind: ClusterIssuer
`,
			expected: []issuerFinding{{certificateName: "ingress-cert", line: 7, value: "letsencrypt"}},
		},
		{
			name: "letsencrypt-gateway prefix concatenated with a template",
			content: `apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: "gateway-cert"
spec:
  issuerRef:
    kind: ClusterIssuer
    name: "letsencrypt-gateway-{{ $moduleGateway.name }}" # gateway issuer
`,
			expected: []issuerFinding{{
				certificateName: "gateway-cert",
				line:            8,
				value:           `"letsencrypt-gateway-{{ $moduleGateway.name }}"`,
			}},
		},
		{
			name: "inline issuerRef",
			content: `apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: inline-cert
spec:
  issuerRef: {name: letsencrypt-staging, kind: ClusterIssuer}
`,
			expected: []issuerFinding{{
				certificateName: "inline-cert",
				line:            6,
				value:           "{name: letsencrypt-staging, kind: ClusterIssuer}",
			}},
		},
		{
			name: "variable assigned a letsencrypt literal",
			content: `{{- $issuerName := printf "letsencrypt-gateway-%s" $moduleGateway.name }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ $issuerName }}
    kind: ClusterIssuer
`,
			expected: []issuerFinding{{
				certificateName: "gateway-cert",
				line:            9,
				value:           `$issuerName := printf "letsencrypt-gateway-%s" $moduleGateway.name`,
			}},
		},
		{
			name: "letsencrypt literal reached through a chain of variables",
			content: `{{- $base := "letsencrypt" }}
{{- $issuerName := printf "%s-gateway-%s" $base $moduleGateway.name }}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ $issuerName | quote }}
    kind: ClusterIssuer
`,
			expected: []issuerFinding{{certificateName: "gateway-cert", line: 9, value: `$base := "letsencrypt"`}},
		},
		{
			name: "every branch of a conditional issuerRef name is checked",
			content: `apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
  {{- if .Values.module.gateway }}
    name: {{ printf "letsencrypt-gateway-%s" .Values.module.gateway }}
  {{- else }}
    name: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name" . }}
  {{- end }}
    kind: ClusterIssuer
`,
			expected: []issuerFinding{{
				certificateName: "gateway-cert",
				line:            8,
				value:           `{{ printf "letsencrypt-gateway-%s" .Values.module.gateway }}`,
			}},
		},
		{
			name: "only the offending Certificate of a multi-document file is reported",
			content: `{{- if (include "helm_lib_module_ingress_enabled" .) }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ingress-cert
spec:
  issuerRef:
    name: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name" . }}
    kind: ClusterIssuer
{{- end }}
{{- if (include "helm_lib_module_gateway_enabled" .) }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
    kind: ClusterIssuer
{{- end }}
`,
			expected: []issuerFinding{{
				certificateName: "gateway-cert",
				line:            20,
				value:           `{{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}`,
			}},
		},
		{
			name: "gateway api helper",
			content: `apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name_for_gateway_api" . }}
    kind: ClusterIssuer
`,
		},
		{
			name: "conditional letsencrypt gateway suffix",
			content: `{{- $moduleGateway := dict }}
{{- include "helm_lib_module_gateway" (list . $moduleGateway) }}
{{- if and (eq (include "helm_lib_module_gateway_enabled" .) "true") (eq (include "helm_lib_module_https_mode" .) "CertManager") }}
{{- $issuerName := include "helm_lib_module_https_cert_manager_cluster_issuer_name" . }}
{{- if or (eq $issuerName "letsencrypt") (eq $issuerName "letsencrypt-staging") }}
  {{- $issuerName = printf "%s-gateway-%s" $issuerName $moduleGateway.name }}
{{- end }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ $issuerName }}
    kind: ClusterIssuer
{{- end }}
`,
		},
		{
			name: "letsencrypt outside Certificate objects",
			content: `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  issuer: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
`,
		},
		{
			name: "letsencrypt only in comments",
			content: `{{- /* issuerRef.name used to be printf "letsencrypt-gateway-%s" */ -}}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    # name: letsencrypt
    name: {{ include "helm_lib_module_https_cert_manager_cluster_issuer_name_for_gateway_api" . }} # not letsencrypt
    kind: ClusterIssuer
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, findHardcodedLetsencryptIssuers(tt.content))
		})
	}
}

func TestCertificateGatewayIssuerRule_Check(t *testing.T) {
	const forbidden = `{{- $moduleGateway := dict }}
{{- if $moduleGateway }}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gateway-cert
spec:
  issuerRef:
    name: {{ printf "letsencrypt-gateway-%s" $moduleGateway.name }}
    kind: ClusterIssuer
{{- end }}
`

	tests := []struct {
		name          string
		templateFiles map[string]string
		excludeRules  []pkg.KindRuleExclude
		expectedFiles []string
	}{
		{
			name: "reports hardcoded issuer in a template that is not rendered",
			templateFiles: map[string]string{
				"templates/gateway/certificate.yaml": forbidden,
				"templates/_helpers.tpl":             `{{- define "noop" }}{{- end }}`,
			},
			expectedFiles: []string{"templates/gateway/certificate.yaml"},
		},
		{
			name: "skips excluded certificate",
			templateFiles: map[string]string{
				"templates/certificate.yaml": forbidden,
			},
			excludeRules:  []pkg.KindRuleExclude{{Kind: "Certificate", Name: "gateway-cert"}},
			expectedFiles: nil,
		},
		{
			name:          "module without templates",
			templateFiles: map[string]string{"openapi/values.yaml": forbidden},
			expectedFiles: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := t.TempDir()

			for filePath, content := range tt.templateFiles {
				fullPath := filepath.Join(modulePath, filePath)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
			}

			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)
			mockModule.GetPathMock.Return(modulePath)

			errorList := errors.NewLintRuleErrorsList()
			NewCertificateGatewayIssuerRule(tt.excludeRules, mockModule, errorList).Check(t.Context())

			errs := errorList.GetErrors()
			require.Len(t, errs, len(tt.expectedFiles))

			for i, expectedFile := range tt.expectedFiles {
				assert.Equal(t, expectedFile, errs[i].FilePath)
				assert.Equal(t, CertificateGatewayIssuerRuleName, errs[i].RuleID)
				assert.Equal(t, "kind = Certificate ; name = gateway-cert", errs[i].ObjectID)
				assert.Equal(t, 9, errs[i].LineNumber)
				assert.Contains(t, errs[i].Text, "pinned to the letsencrypt ClusterIssuer")
				assert.Contains(t, errs[i].Text, gatewayAPIClusterIssuerHelper)
			}
		})
	}
}
