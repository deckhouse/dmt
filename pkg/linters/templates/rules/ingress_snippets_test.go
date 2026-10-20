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
	"fmt"
	"os"
	"path/filepath"
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

const (
	configurationSnippetAnnotation = nginxAnnotationPrefix + "configuration-snippet"
	snippetValue                   = "proxy_set_header X-Test value;"
)

var snippetAnnotations = []string{
	configurationSnippetAnnotation,
	nginxAnnotationPrefix + "server-snippet",
	nginxAnnotationPrefix + "auth-snippet",
	nginxAnnotationPrefix + "modsecurity-snippet",
	nginxAnnotationPrefix + "stream-snippet",
}

func TestIngressSnippetsRuleReportsUnguardedSnippetAnnotations(t *testing.T) {
	for _, annotation := range snippetAnnotations {
		t.Run(annotation, func(t *testing.T) {
			findings := runIngressSnippetsRule(t,
				"Ingress",
				ingressTemplate(annotationLine(annotation, snippetValue)),
				nil, nil,
			)

			require.Len(t, findings, 1)
			assert.Equal(t, pkg.Error, findings[0].Level)
			assert.Contains(t, findings[0].Text, annotation)
			assert.Contains(t, findings[0].Text, "discouraged to use in DKP modules")
			assert.Contains(t, findings[0].Text, "legacy versions of the ingress-nginx controller")
			assert.Contains(t, findings[0].Text, "user namespaces")
			assert.Contains(t, findings[0].Text, "explicitly allows them at the controller level")
			assert.Contains(t, findings[0].Text, ingressNginxAnnotationsVersionHelper)
			assert.Contains(t, findings[0].Text, ingressNginxAnnotationsVersionLibHelm)
			assert.Contains(t, findings[0].Text, snippetAnnotationsDocsURL)
		})
	}
}

func TestIngressSnippetsRuleAcceptsGuardedSnippetAnnotations(t *testing.T) {
	conditions := []string{
		fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, "1.3.0"),
		fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, "1.4.2"),
		fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, "v1.3.0"),
		fmt.Sprintf(`{{- if not (include %q (list $ %q)) }}`, ingressNginxAnnotationsVersionHelper, "1.3.0"),
		fmt.Sprintf(`{{- if and (include %q (list $ %q)) (eq .Values.foo "bar") }}`, ingressNginxAnnotationsVersionHelper, "1.5.0"),
	}

	for _, condition := range conditions {
		t.Run(condition, func(t *testing.T) {
			findings := runIngressSnippetsRule(t,
				"Ingress",
				ingressTemplate(
					condition+"\n"+
						annotationLine(configurationSnippetAnnotation, snippetValue)+
						"{{- end }}\n",
				),
				nil, nil,
			)

			assert.Empty(t, findings)
		})
	}
}

func TestIngressSnippetsRuleAcceptsSnippetInGuardedElseBranch(t *testing.T) {
	condition := fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, "1.3.0")

	findings := runIngressSnippetsRule(t,
		"Ingress",
		ingressTemplate(
			condition+"\n"+
				"    "+nginxAnnotationPrefix+"proxy-ssl-use-controller-certificate: \"true\"\n"+
				"{{- else }}\n"+
				annotationLine(configurationSnippetAnnotation, snippetValue)+
				"{{- end }}\n",
		),
		nil, nil,
	)

	assert.Empty(t, findings)
}

func TestIngressSnippetsRuleAcceptsSnippetNestedInGuardedBlock(t *testing.T) {
	condition := fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, "1.3.0")

	findings := runIngressSnippetsRule(t,
		"Ingress",
		ingressTemplate(
			condition+"\n"+
				"{{- with .Values.foo }}\n"+
				annotationLine(configurationSnippetAnnotation, snippetValue)+
				"{{- end }}\n"+
				"{{- end }}\n",
		),
		nil, nil,
	)

	assert.Empty(t, findings)
}

func TestIngressSnippetsRuleRejectsInvalidGuardVersions(t *testing.T) {
	for _, version := range []string{"1.2.9", "1.3.0-rc.1", "not-a-version"} {
		t.Run(version, func(t *testing.T) {
			condition := fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, version)

			findings := runIngressSnippetsRule(t,
				"Ingress",
				ingressTemplate(
					condition+"\n"+
						annotationLine(configurationSnippetAnnotation, snippetValue)+
						"{{- end }}\n",
				),
				nil, nil,
			)

			require.Len(t, findings, 1)
			assert.Contains(t, findings[0].Text, "discouraged to use in DKP modules")
		})
	}
}

func TestIngressSnippetsRuleReportsSnippetOutsideGuard(t *testing.T) {
	condition := fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, "1.3.0")

	findings := runIngressSnippetsRule(t,
		"Ingress",
		ingressTemplate(
			condition+"\n"+
				"{{- end }}\n"+
				annotationLine(configurationSnippetAnnotation, snippetValue),
		),
		nil, nil,
	)

	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Text, "discouraged to use in DKP modules")
}

func TestIngressSnippetsRuleReportsOnlyUnguardedOccurrences(t *testing.T) {
	condition := fmt.Sprintf(`{{- if include %q (list $ %q) }}`, ingressNginxAnnotationsVersionHelper, "1.3.0")
	serverSnippet := nginxAnnotationPrefix + "server-snippet"

	findings := runIngressSnippetsRule(t,
		"Ingress",
		ingressTemplate(
			condition+"\n"+
				annotationLine(configurationSnippetAnnotation, snippetValue)+
				"{{- end }}\n"+
				annotationLine(serverSnippet, "return 200;"),
		),
		nil, nil,
	)

	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Text, serverSnippet)
}

func TestIngressSnippetsRuleIgnoresCommentedSnippetAnnotation(t *testing.T) {
	findings := runIngressSnippetsRule(t,
		"Ingress",
		ingressTemplate("# "+configurationSnippetAnnotation+": value\n"),
		nil, nil,
	)

	assert.Empty(t, findings)
}

func TestIngressSnippetsRuleSkipsExcludedFilesAndDirectories(t *testing.T) {
	content := ingressTemplate(annotationLine(configurationSnippetAnnotation, snippetValue))

	tests := []struct {
		name        string
		files       []pkg.StringRuleExclude
		directories []pkg.DirectoryRuleExclude
	}{
		{
			name:  "excluded file",
			files: []pkg.StringRuleExclude{"templates/ingress.yaml"},
		},
		{
			name:        "excluded directory",
			directories: []pkg.DirectoryRuleExclude{"templates"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := runIngressSnippetsRule(t, "Ingress", content, tt.files, tt.directories)
			assert.Empty(t, findings)
		})
	}

	t.Run("unrelated exclusion does not skip the file", func(t *testing.T) {
		findings := runIngressSnippetsRule(t, "Ingress", content,
			[]pkg.StringRuleExclude{"templates/other.yaml"},
			[]pkg.DirectoryRuleExclude{"templates/legacy"},
		)

		require.Len(t, findings, 1)
	})
}

func TestIngressSnippetsRuleSkipsNonIngressResources(t *testing.T) {
	content := ingressTemplate(annotationLine(configurationSnippetAnnotation, snippetValue))

	assert.Empty(t, runIngressSnippetsRule(t, "Deployment", content, nil, nil))
}

func ingressTemplate(annotationLines string) string {
	return `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test
  annotations:
` + annotationLines + `spec: {}
`
}

func annotationLine(annotation, value string) string {
	return fmt.Sprintf("    %s: %q\n", annotation, value)
}

func runIngressSnippetsRule(
	t *testing.T,
	kind string,
	template string,
	excludeFiles []pkg.StringRuleExclude,
	excludeDirectories []pkg.DirectoryRuleExclude,
) []pkg.LinterError {
	t.Helper()

	moduleDir := t.TempDir()
	templatesDir := filepath.Join(moduleDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	templatePath := filepath.Join(templatesDir, "ingress.yaml")
	require.NoError(t, os.WriteFile(templatePath, []byte(template), 0o644))

	object := unstructured.Unstructured{}
	object.SetAPIVersion("networking.k8s.io/v1")
	object.SetKind(kind)
	object.SetName("test")

	objects := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: kind, Name: object.GetName()}: {
			Unstructured: object,
			AbsPath:      templatePath,
		},
	}

	module := mocks.NewModuleMock(minimock.NewController(t))
	module.GetStorageMock.Return(objects)
	module.GetPathMock.Optional().Return(moduleDir)

	errorList := errors.NewLintRuleErrorsList()
	NewIngressSnippetsRule(excludeFiles, excludeDirectories, module, errorList).Check(t.Context())

	return errorList.GetErrors()
}
