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

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestIngressRuleWarnsAboutUnsafeAnnotations(t *testing.T) {
	for _, annotation := range unsafeIngressAnnotations {
		t.Run(annotation, func(t *testing.T) {
			value := "value"
			if annotation == configurationSnippetAnnotation {
				value = legacyHSTSDirective
			}

			findings := runIngressRule(t, "Ingress", map[string]string{annotation: value}, nil)

			require.Len(t, findings, 1)
			assert.Equal(t, pkg.Warn, findings[0].Level)
			assert.Contains(t, findings[0].Text, annotation)
		})
	}
}

func TestIngressRuleHSTSCompatibility(t *testing.T) {
	tests := []struct {
		name         string
		annotations  map[string]string
		wantWarnings int
		wantErrors   int
	}{
		{
			name: "safe HSTS annotation",
			annotations: map[string]string{
				configurationSnippetAnnotation: "proxy_set_header X-Test value;",
				ingressNginxHSTSAnnotation:     "true",
			},
			wantWarnings: 1,
		},
		{
			name: "legacy HSTS directive",
			annotations: map[string]string{
				configurationSnippetAnnotation: legacyHSTSDirective + " value;",
			},
			wantWarnings: 1,
		},
		{
			name: "HSTS is missing",
			annotations: map[string]string{
				configurationSnippetAnnotation: "proxy_set_header X-Test value;",
			},
			wantWarnings: 1,
			wantErrors:   1,
		},
		{
			name: "configuration snippet is empty",
			annotations: map[string]string{
				configurationSnippetAnnotation: "",
			},
			wantWarnings: 1,
			wantErrors:   1,
		},
		{
			name: "safe HSTS annotation is false",
			annotations: map[string]string{
				configurationSnippetAnnotation: "proxy_set_header X-Test value;",
				ingressNginxHSTSAnnotation:     "false",
			},
			wantWarnings: 1,
			wantErrors:   1,
		},
		{
			name: "safe HSTS annotation is empty",
			annotations: map[string]string{
				configurationSnippetAnnotation: "proxy_set_header X-Test value;",
				ingressNginxHSTSAnnotation:     "",
			},
			wantWarnings: 1,
			wantErrors:   1,
		},
		{
			name: "safe HSTS annotation has another value",
			annotations: map[string]string{
				configurationSnippetAnnotation: "proxy_set_header X-Test value;",
				ingressNginxHSTSAnnotation:     "TRUE",
			},
			wantWarnings: 1,
			wantErrors:   1,
		},
		{
			name: "safe annotations without configuration snippet",
			annotations: map[string]string{
				ingressNginxHSTSAnnotation:                                     "true",
				nginxAnnotationPrefix + "proxy-ssl-use-controller-certificate": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := runIngressRule(t, "Ingress", tt.annotations, nil)

			var warnings, ruleErrors int

			for _, finding := range findings {
				switch finding.Level {
				case pkg.Warn:
					warnings++
				case pkg.Error:
					ruleErrors++

					assert.Contains(t, finding.Text, ingressNginxHSTSAnnotation)
				}
			}

			assert.Equal(t, tt.wantWarnings, warnings)
			assert.Equal(t, tt.wantErrors, ruleErrors)
		})
	}
}

func TestIngressRuleReportsUnsafeAnnotationsInStableOrder(t *testing.T) {
	findings := runIngressRule(t, "Ingress", map[string]string{
		nginxAnnotationPrefix + "stream-snippet": "stream {}",
		nginxAnnotationPrefix + "server-snippet": "return 200;",
	}, nil)

	require.Len(t, findings, 2)
	assert.Contains(t, findings[0].Text, nginxAnnotationPrefix+"server-snippet")
	assert.Contains(t, findings[1].Text, nginxAnnotationPrefix+"stream-snippet")
}

func TestIngressRuleSkipsExcludedAndNonIngressResources(t *testing.T) {
	exclude := []pkg.KindRuleExclude{{Kind: "Ingress", Name: "test"}}
	annotations := map[string]string{configurationSnippetAnnotation: "value"}

	assert.Empty(t, runIngressRule(t, "Ingress", annotations, exclude))
	assert.Empty(t, runIngressRule(t, "Deployment", annotations, nil))
}

func runIngressRule(t *testing.T, kind string, annotations map[string]string, exclude []pkg.KindRuleExclude) []pkg.LinterError {
	t.Helper()

	object := unstructured.Unstructured{}
	object.SetAPIVersion("networking.k8s.io/v1")
	object.SetKind(kind)
	object.SetName("test")
	object.SetAnnotations(annotations)

	objects := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: kind, Name: object.GetName()}: {
			Unstructured: object,
			AbsPath:      "/test/ingress.yaml",
		},
	}

	module := mocks.NewModuleMock(minimock.NewController(t))
	module.GetStorageMock.Return(objects)

	errorList := errors.NewLintRuleErrorsList()
	NewIngressRule(exclude, module, errorList).Check(t.Context())

	return errorList.GetErrors()
}
