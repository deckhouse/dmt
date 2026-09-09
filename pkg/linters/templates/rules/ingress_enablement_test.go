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

func TestIngressEnablementRule_Check(t *testing.T) {
	const ungatedIngress = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: d8-my-module
spec:
  rules:
  - host: dashboard.example.com
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
			name: "flags an Ingress with no enablement check at all",
			templateFiles: map[string]string{
				"templates/ingress.yaml": ungatedIngress,
			},
			storageKinds: []string{"Ingress"},
			wantCount:    1,
			wantContains: []string{
				"helm_lib_module_ingress_enabled", "Ingress",
				"global.modules.ingress.enabled", "myModule.ingress.enabled",
			},
			wantLines: []int{2},
		},
		{
			name: "passes an Ingress guarded by helm_lib_module_ingress_enabled in the same file",
			templateFiles: map[string]string{
				"templates/ingress.yaml": `{{- if eq (include "helm_lib_module_ingress_enabled" .) "true" }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: d8-my-module
spec:
  rules:
  - host: dashboard.example.com
{{- end }}
`,
			},
			storageKinds: []string{"Ingress"},
			wantCount:    0,
		},
		{
			name: "ignores files that never create an Ingress",
			templateFiles: map[string]string{
				"templates/deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
`,
			},
			storageKinds: []string{"Ingress"}, // module has one, just not from this file
			wantCount:    0,
		},
		{
			name: "does not confuse an unrelated kind field with Ingress",
			templateFiles: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  note: "this is not an IngressClass"
`,
			},
			storageKinds: []string{"Ingress"},
			wantCount:    0,
		},
		{
			name: "does not run at all when the module ships no Ingress",
			templateFiles: map[string]string{
				"templates/ingress.yaml": ungatedIngress,
			},
			storageKinds: nil, // e.g. a module whose Ingress never actually rendered
			wantCount:    0,
		},
		{
			name: "an excluded file is skipped even though the module ships an Ingress",
			templateFiles: map[string]string{
				"templates/ingress.yaml": ungatedIngress,
			},
			storageKinds: []string{"Ingress"},
			exclude:      []pkg.StringRuleExclude{"templates/ingress.yaml"},
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := writeTemplatesModule(t, tt.templateFiles)

			errorList := errors.NewLintRuleErrorsList()
			NewIngressEnablementRule(tt.exclude, nil, templatesMockModule(t, modulePath, tt.storageKinds...), errorList).Check(t.Context())

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

func TestIngressEnablementRule_DirectoryExclusion(t *testing.T) {
	modulePath := writeTemplatesModule(t, map[string]string{
		"templates/vendor/ingress.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
`,
	})

	errorList := errors.NewLintRuleErrorsList()
	NewIngressEnablementRule(
		nil,
		[]pkg.DirectoryRuleExclude{"templates/vendor/"},
		templatesMockModule(t, modulePath, "Ingress"),
		errorList,
	).Check(t.Context())

	require.Empty(t, errorList.GetErrors())
}
