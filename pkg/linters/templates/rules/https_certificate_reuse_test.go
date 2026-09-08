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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestHTTPSCertificateReuseRule_Check(t *testing.T) {
	const ingressUsingSharedSecret = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
spec:
  tls:
    - secretName: {{ include "helm_lib_module_https_secret_name" (list . "my-module-ingress-tls") }}
`
	const ingressNoSecretRef = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
`
	const httprouteLinkedToSharedSecret = `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
spec:
  hostnames:
    - dashboard.example.com
  tls:
    secretRef:
      name: {{ include "helm_lib_module_https_secret_name" (list . "my-module-ingress-tls" "my-module-httproute-tls") }}
`
	const copyShared = `{{- include "helm_lib_module_https_copy_custom_certificate" (list . "d8-my-module" "my-module-ingress-tls") }}
`
	const copyOverride = `{{- include "helm_lib_module_https_copy_custom_certificate" (list . "d8-my-module" "my-module-httproute-tls") }}
`

	tests := []struct {
		name          string
		templateFiles map[string]string
		storageKinds  []string
		exclude       []pkg.StringRuleExclude
		wantCount     int
		wantContains  []string
		wantLines     []int
	}{
		{
			name: "passes the canonical correct usage: copied once, ingress plain, gateway linked",
			templateFiles: map[string]string{
				"templates/ingress.yaml":            ingressUsingSharedSecret,
				"templates/custom-certificate.yaml": copyShared,
				"templates/httproute.yaml":          httprouteLinkedToSharedSecret,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    0,
		},
		{
			// Real-world reproduction: templates/httproute.yaml bundles the
			// HTTPRoute together with the cert-manager Certificate that
			// feeds it, separated by "---". The Certificate's own secretName
			// legitimately uses the plain form (it names a NEW target secret
			// for cert-manager, not an existing shared one) and must not be
			// classified as "the Gateway API flow's own TLS secret
			// reference" just because the file also contains a HTTPRoute.
			name: "passes a plain-form reference inside a Certificate document bundled with a HTTPRoute",
			templateFiles: map[string]string{
				"templates/ingress.yaml":            ingressUsingSharedSecret,
				"templates/custom-certificate.yaml": copyShared,
				"templates/httproute.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
spec:
  tls:
    secretRef:
      name: {{ include "helm_lib_module_https_secret_name" (list . "my-module-ingress-tls" "my-module-httproute-tls") }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: dashboard-httproute
spec:
  secretName: {{ include "helm_lib_module_https_secret_name" (list . "my-module-httproute-tls") }}
`,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    0,
		},
		{
			name: "flags the override prefix also being independently copied",
			templateFiles: map[string]string{
				"templates/ingress.yaml":            ingressUsingSharedSecret,
				"templates/custom-certificate.yaml": copyShared + copyOverride,
				"templates/httproute.yaml":          httprouteLinkedToSharedSecret,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    1,
			wantContains: []string{
				"my-module-httproute-tls", "my-module-ingress-tls",
				"helm_lib_module_https_secret_name", "duplicate",
			},
			wantLines: []int{2},
		},
		{
			name: "flags a secret prefix copied by two separate copy calls",
			templateFiles: map[string]string{
				"templates/ingress.yaml":       ingressUsingSharedSecret,
				"templates/custom-cert-a.yaml": copyShared,
				"templates/custom-cert-b.yaml": copyShared,
				"templates/httproute.yaml":     httprouteLinkedToSharedSecret,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    1,
			wantContains: []string{"my-module-ingress-tls", "more than one", "only one copy is needed"},
		},
		{
			name: "flags a HTTPRoute reference using the plain form instead of the two-prefix form",
			templateFiles: map[string]string{
				"templates/ingress.yaml":            ingressUsingSharedSecret,
				"templates/custom-certificate.yaml": copyShared,
				"templates/httproute.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
spec:
  tls:
    secretRef:
      name: {{ include "helm_lib_module_https_secret_name" (list . "my-module-httproute-tls") }}
`,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    1,
			wantContains: []string{"my-module-httproute-tls", "plain form", "extended form"},
		},
		{
			name: "flags a ListenerSet reference using the plain form (real-world istio/api-proxy reproduction)",
			templateFiles: map[string]string{
				"templates/ingress.yaml": ingressNoSecretRef,
				"templates/custom-certificate.yaml": `{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "istio-ingress-tls") }}
{{ $moduleGateway := dict }}
{{ include "helm_lib_module_gateway" (list . $moduleGateway) }}
{{ if $moduleGateway }}
{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "istio-httproute-tls") }}
{{ if .Values.istio.multicluster.enabled }}
{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "api-proxy-httproute-tls") }}
{{ end }}
{{ end }}
{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "api-proxy-ingress-tls") }}
`,
				"templates/listenerset.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: istio
spec:
  listeners:
  - tls:
      certificateRefs:
      - name: {{ include "helm_lib_module_https_secret_name" (list . "istio-httproute-tls") }}
`,
				"templates/multicluster/api-proxy/listenerset.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: api-proxy
spec:
  listeners:
  - tls:
      certificateRefs:
      - name: {{ include "helm_lib_module_https_secret_name" (list . "api-proxy-httproute-tls") }}
`,
			},
			storageKinds: []string{"Ingress", "ListenerSet"},
			// Both signals fire here, at four distinct locations: the two
			// ListenerSet references using the plain form (check 3), and the
			// two "-httproute-tls" copies flagged separately by the naming
			// convention check (check 6), since it can't tell they're the
			// same underlying problem as the bad references above.
			wantCount: 4,
			wantContains: []string{
				"istio-httproute-tls", "api-proxy-httproute-tls", "plain form", "extended form",
				"naming convention",
			},
		},
		{
			name: "flags per-stem duplicate copies from the copy calls alone, with no secret_name reference anywhere",
			templateFiles: map[string]string{
				"templates/ingress.yaml": ingressNoSecretRef,
				"templates/custom-certificate.yaml": `{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "istio-ingress-tls") }}
{{ $moduleGateway := dict }}
{{ include "helm_lib_module_gateway" (list . $moduleGateway) }}
{{ if $moduleGateway }}
{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "istio-httproute-tls") }}
{{ if .Values.istio.multicluster.enabled }}
{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "api-proxy-httproute-tls") }}
{{ end }}
{{ end }}
{{ include "helm_lib_module_https_copy_custom_certificate" (list . "d8-istio" "api-proxy-ingress-tls") }}
`,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    2,
			wantContains: []string{
				"istio-httproute-tls", "istio-ingress-tls", "naming convention",
				"api-proxy-httproute-tls", "api-proxy-ingress-tls",
			},
		},
		{
			// The rule used to also flag a HTTPRoute link whose base didn't
			// match any prefix an Ingress file referenced, but that produced
			// false positives when a module's Ingress and Gateway API
			// manifests for the same logical certificate live in separate
			// directories the rule doesn't otherwise correlate — removed.
			name: "passes a HTTPRoute link whose base the Ingress flow never references",
			templateFiles: map[string]string{
				"templates/ingress.yaml":            ingressUsingSharedSecret,
				"templates/custom-certificate.yaml": copyShared,
				"templates/httproute.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
spec:
  tls:
    secretRef:
      name: {{ include "helm_lib_module_https_secret_name" (list . "some-other-secret" "some-other-secret-httproute") }}
`,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    0,
		},
		{
			name: "flags an Ingress reference using the two-prefix form",
			templateFiles: map[string]string{
				"templates/ingress.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
spec:
  tls:
    - secretName: {{ include "helm_lib_module_https_secret_name" (list . "my-module-ingress-tls" "my-module-httproute-tls") }}
`,
				"templates/custom-certificate.yaml": copyShared,
				"templates/httproute.yaml":          httprouteLinkedToSharedSecret,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			wantCount:    1,
			wantContains: []string{"my-module-ingress-tls", "plain form", "no Gateway-API-specific override"},
		},
		{
			name: "passes two unrelated certificates, each copied once and correctly linked, for two different services",
			templateFiles: map[string]string{
				"templates/dex/ingress.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dex
spec:
  tls:
    - secretName: {{ include "helm_lib_module_https_secret_name" (list . "ingress-tls") }}
`,
				"templates/kubeconfig-generator/ingress.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kubeconfig
spec:
  tls:
    - secretName: {{ include "helm_lib_module_https_secret_name" (list . "kubeconfig-ingress-tls") }}
`,
				"templates/custom-certificate.yaml": `{{- include "helm_lib_module_https_copy_custom_certificate" (list . "d8-user-authn" "ingress-tls") }}
`,
				"templates/kubeconfig-generator/custom-certificate.yaml": `{{- include "helm_lib_module_https_copy_custom_certificate" (list . "d8-user-authn" "kubeconfig-ingress-tls") }}
`,
				"templates/listenerset.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: user-authn
spec:
  listeners:
  - tls:
      certificateRefs:
      - name: {{ include "helm_lib_module_https_secret_name" (list . "ingress-tls" "httproute-tls") }}
  - tls:
      certificateRefs:
      - name: {{ include "helm_lib_module_https_secret_name" (list . "kubeconfig-ingress-tls" "kubeconfig-httproute-tls") }}
`,
			},
			storageKinds: []string{"Ingress", "ListenerSet"},
			wantCount:    0,
		},
		{
			name: "does not run when the module has no Ingress",
			templateFiles: map[string]string{
				"templates/custom-certificate.yaml": copyShared + copyOverride,
				"templates/httproute.yaml":          httprouteLinkedToSharedSecret,
			},
			storageKinds: []string{"HTTPRoute"},
			wantCount:    0,
		},
		{
			name: "does not run when the module has no Gateway API resource",
			templateFiles: map[string]string{
				"templates/ingress.yaml":            ingressUsingSharedSecret,
				"templates/custom-certificate.yaml": copyShared + copyOverride,
			},
			storageKinds: []string{"Ingress"},
			wantCount:    0,
		},
		{
			name: "an excluded file is skipped even though it copies the duplicate certificate",
			templateFiles: map[string]string{
				"templates/ingress.yaml":            ingressUsingSharedSecret,
				"templates/custom-certificate.yaml": copyShared + copyOverride,
				"templates/httproute.yaml":          httprouteLinkedToSharedSecret,
			},
			storageKinds: []string{"Ingress", "HTTPRoute"},
			exclude:      []pkg.StringRuleExclude{"templates/custom-certificate.yaml"},
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := writeTemplatesModule(t, tt.templateFiles)

			errorList := errors.NewLintRuleErrorsList()
			NewHTTPSCertificateReuseRule(tt.exclude, nil, templatesMockModule(t, modulePath, tt.storageKinds...), errorList).Check(t.Context())

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

				require.Truef(t, found, "expected a finding containing %q, got %+v", want, errs)
			}

			for i, wantLine := range tt.wantLines {
				if i < len(errs) {
					require.Equalf(t, wantLine, errs[i].LineNumber, "unexpected line for finding %d: %s", i, errs[i].Text)
				}
			}
		})
	}
}

func TestHTTPSCertificateReuseRule_DirectoryExclusion(t *testing.T) {
	modulePath := writeTemplatesModule(t, map[string]string{
		"templates/ingress.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
spec:
  tls:
    - secretName: {{ include "helm_lib_module_https_secret_name" (list . "my-module-ingress-tls") }}
`,
		"templates/vendor/custom-certificate.yaml": `{{- include "helm_lib_module_https_copy_custom_certificate" (list . "d8-my-module" "my-module-ingress-tls") }}
{{- include "helm_lib_module_https_copy_custom_certificate" (list . "d8-my-module" "my-module-httproute-tls") }}
`,
		"templates/httproute.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
spec:
  tls:
    secretRef:
      name: {{ include "helm_lib_module_https_secret_name" (list . "my-module-ingress-tls" "my-module-httproute-tls") }}
`,
	})

	errorList := errors.NewLintRuleErrorsList()
	NewHTTPSCertificateReuseRule(
		nil,
		[]pkg.DirectoryRuleExclude{"templates/vendor/"},
		templatesMockModule(t, modulePath, "Ingress", "HTTPRoute"),
		errorList,
	).Check(t.Context())

	require.Empty(t, errorList.GetErrors())
}
