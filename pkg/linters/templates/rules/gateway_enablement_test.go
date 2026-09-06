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

func TestGatewayEnablementRule_Check(t *testing.T) {
	const ungatedHTTPRoute = `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
  namespace: d8-my-module
`

	const ungatedListenerSet = `apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: dashboard
  namespace: d8-my-module
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
			name: "flags an HTTPRoute with no enablement check at all",
			templateFiles: map[string]string{
				"templates/httproute.yaml": ungatedHTTPRoute,
			},
			storageKinds: []string{"HTTPRoute"},
			wantCount:    1,
			wantContains: []string{
				"helm_lib_module_gateway_enabled", "Gateway API",
				"global.modules.gatewayAPI.enabled", "myModule.gatewayAPI.enabled",
				"global.discovery.gatewayAPIDefaultGateway",
			},
			wantLines: []int{2},
		},
		{
			name: "flags a ListenerSet with no enablement check at all",
			templateFiles: map[string]string{
				"templates/listenerset.yaml": ungatedListenerSet,
			},
			storageKinds: []string{"ListenerSet"},
			wantCount:    1,
		},
		{
			name: "passes a ListenerSet and HTTPRoute guarded by helm_lib_module_gateway_enabled in the same file",
			templateFiles: map[string]string{
				"templates/httproute.yaml": `{{- if and (eq (include "helm_lib_module_gateway_enabled" .) "true") .Values.global.modules.publicDomainTemplate }}
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: dashboard
  namespace: d8-my-module
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
  namespace: d8-my-module
{{- end }}
`,
			},
			storageKinds: []string{"HTTPRoute", "ListenerSet"},
			wantCount:    0,
		},
		{
			name: "ignores files that never create Gateway API objects",
			templateFiles: map[string]string{
				"templates/ingress.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
`,
			},
			storageKinds: []string{"HTTPRoute"}, // module has one, just not from this file
			wantCount:    0,
		},
		{
			name: "does not run at all when the module ships neither HTTPRoute nor ListenerSet",
			templateFiles: map[string]string{
				"templates/httproute.yaml": ungatedHTTPRoute,
			},
			storageKinds: nil, // e.g. a module whose HTTPRoute never actually rendered
			wantCount:    0,
		},
		{
			name: "an excluded file is skipped even though the module ships an HTTPRoute",
			templateFiles: map[string]string{
				"templates/httproute.yaml": ungatedHTTPRoute,
			},
			storageKinds: []string{"HTTPRoute"},
			exclude:      []pkg.StringRuleExclude{"templates/httproute.yaml"},
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := writeTemplatesModule(t, tt.templateFiles)

			errorList := errors.NewLintRuleErrorsList()
			NewGatewayEnablementRule(tt.exclude, nil, templatesMockModule(t, modulePath, tt.storageKinds...), errorList).Check(t.Context())

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

func TestGatewayEnablementRule_DirectoryExclusion(t *testing.T) {
	modulePath := writeTemplatesModule(t, map[string]string{
		"templates/vendor/httproute.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: dashboard
`,
	})

	errorList := errors.NewLintRuleErrorsList()
	NewGatewayEnablementRule(
		nil,
		[]pkg.DirectoryRuleExclude{"templates/vendor/"},
		templatesMockModule(t, modulePath, "HTTPRoute"),
		errorList,
	).Check(t.Context())

	require.Empty(t, errorList.GetErrors())
}
