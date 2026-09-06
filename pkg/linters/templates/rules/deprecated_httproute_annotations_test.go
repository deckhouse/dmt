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
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// writeTemplatesModule builds a temporary module directory containing the given
// template files (keyed by path relative to the module root) and returns the
// module path.
func writeTemplatesModule(t *testing.T, templateFiles map[string]string) string {
	t.Helper()

	modulePath := filepath.Join(t.TempDir(), "module")
	require.NoError(t, os.MkdirAll(modulePath, 0o755))

	for relPath, content := range templateFiles {
		fullPath := filepath.Join(modulePath, relPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
	}

	return modulePath
}

// kindOnlyStorage builds a minimal rendered-object store containing one bare
// object per kind given — enough for storageHasKind to see them, which is all
// these rules read from GetStorage().
func kindOnlyStorage(kinds ...string) map[storage.ResourceIndex]storage.StoreObject {
	out := make(map[storage.ResourceIndex]storage.StoreObject, len(kinds))

	for i, kind := range kinds {
		u := unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": fmt.Sprintf("obj-%d", i)},
		}}

		idx := storage.ResourceIndex{Kind: u.GetKind(), Name: u.GetName(), Namespace: u.GetNamespace()}
		out[idx] = storage.StoreObject{Unstructured: u}
	}

	return out
}

// templatesMockModule builds a Module mock rooted at modulePath whose rendered
// storage contains one bare object per kind in storageKinds.
func templatesMockModule(t *testing.T, modulePath string, storageKinds ...string) *mocks.ModuleMock {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	// Optional: the storage gate in each rule's Check may return before GetPath
	// or GetName is ever called (see the "does not run at all" test cases).
	m.GetPathMock.Optional().Return(modulePath)
	m.GetNameMock.Optional().Return("my-module")
	m.GetStorageMock.Return(kindOnlyStorage(storageKinds...))

	return m
}

func TestDeprecatedHTTPRouteAnnotationsRule_Check(t *testing.T) {
	const httprouteWithAnnotation = `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: x
  annotations:
    alb.network.deckhouse.io/response-headers-to-add: '{"Strict-Transport-Security":"max-age=31536000"}'
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
			name: "flags the deprecated response-headers-to-add annotation",
			templateFiles: map[string]string{
				"templates/httproute.yaml": httprouteWithAnnotation,
			},
			storageKinds: []string{"HTTPRoute"},
			wantCount:    1,
			wantContains: []string{
				`alb.network.deckhouse.io/response-headers-to-add`,
				"ResponseHeaderModifier",
				"Strict-Transport-Security",
				"responseHeaderModifier",
			},
			wantLines: []int{6},
		},
		{
			name: "flags multiple occurrences across files",
			templateFiles: map[string]string{
				"templates/a.yaml": `metadata:
  annotations:
    alb.network.deckhouse.io/response-headers-to-add: '{}'
`,
				"templates/b.yaml": `metadata:
  annotations:
    alb.network.deckhouse.io/response-headers-to-add: '{}'
`,
			},
			storageKinds: []string{"HTTPRoute"},
			wantCount:    2,
		},
		{
			name: "ignores files that never use the annotation",
			templateFiles: map[string]string{
				"templates/httproute.yaml": `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: x
  annotations:
    alb.network.deckhouse.io/backend-tls-settings: '{"mode": "SIMPLE"}'
`,
			},
			storageKinds: []string{"HTTPRoute"},
			wantCount:    0,
		},
		{
			name: "does not run at all when the module ships no Ingress/HTTPRoute/ListenerSet",
			templateFiles: map[string]string{
				"templates/httproute.yaml": httprouteWithAnnotation,
			},
			storageKinds: nil, // e.g. a module whose only resources are a Deployment and a Service
			wantCount:    0,
		},
		{
			name: "an excluded file is skipped even though the module ships HTTPRoute",
			templateFiles: map[string]string{
				"templates/httproute.yaml": httprouteWithAnnotation,
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
			NewDeprecatedHTTPRouteAnnotationsRule(tt.exclude, nil, templatesMockModule(t, modulePath, tt.storageKinds...), errorList).Check(t.Context())

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

func TestDeprecatedHTTPRouteAnnotationsRule_DirectoryExclusion(t *testing.T) {
	modulePath := writeTemplatesModule(t, map[string]string{
		"templates/vendor/httproute.yaml": `metadata:
  annotations:
    alb.network.deckhouse.io/response-headers-to-add: '{}'
`,
	})

	errorList := errors.NewLintRuleErrorsList()
	NewDeprecatedHTTPRouteAnnotationsRule(
		nil,
		[]pkg.DirectoryRuleExclude{"templates/vendor/"},
		templatesMockModule(t, modulePath, "HTTPRoute"),
		errorList,
	).Check(t.Context())

	require.Empty(t, errorList.GetErrors())
}
