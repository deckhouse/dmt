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

func schemaStorage(objects ...map[string]any) map[storage.ResourceIndex]storage.StoreObject {
	out := make(map[storage.ResourceIndex]storage.StoreObject, len(objects))

	for i := range objects {
		u := unstructured.Unstructured{Object: objects[i]}

		idx := storage.ResourceIndex{
			Kind:      u.GetKind(),
			Name:      u.GetName(),
			Namespace: u.GetNamespace(),
		}

		out[idx] = storage.StoreObject{Unstructured: u, AbsPath: "/test/" + u.GetName() + ".yaml"}
	}

	return out
}

func runSchemaRule(t *testing.T, exclude []pkg.KindRuleExclude, objects ...map[string]any) *errors.LintRuleErrorsList {
	t.Helper()

	mc := minimock.NewController(t)

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(schemaStorage(objects...))

	errorList := errors.NewLintRuleErrorsList()
	NewSchemaValidationRule(exclude, mod, errorList).Check(t.Context())

	return errorList
}

func TestSchemaValidationRule_ValidService(t *testing.T) {
	errorList := runSchemaRule(t, nil, map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]any{"name": "svc"},
		"spec": map[string]any{
			"ports": []any{map[string]any{"port": int64(80)}},
		},
	})

	assert.False(t, errorList.ContainsErrors(), "valid Service should not produce errors")
}

// TestSchemaValidationRule_UnknownFields covers the strict half of the decode:
// fields the API does not declare. Two of them in one object must come back as two
// findings, not one lump, so each names the field a reader has to go fix.
func TestSchemaValidationRule_UnknownFields(t *testing.T) {
	errorList := runSchemaRule(t, nil, map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]any{"name": "svc"},
		"spec": map[string]any{
			"ports":        []any{map[string]any{"port": int64(80)}},
			"bogusField":   true,
			"anotherBogus": "x",
		},
	})

	errs := errorList.GetErrors()
	assert.Len(t, errs, 2, "each undeclared field must be reported on its own")

	texts := make([]string, 0, len(errs))
	for _, e := range errs {
		texts = append(texts, e.Text)
	}

	assert.Contains(t, strings.Join(texts, "\n"), `unknown field "spec.bogusField"`)
	assert.Contains(t, strings.Join(texts, "\n"), `unknown field "spec.anotherBogus"`)
}

func TestSchemaValidationRule_InvalidService(t *testing.T) {
	errorList := runSchemaRule(t, nil, map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]any{"name": "svc"},
		"spec": map[string]any{
			"ports": []any{map[string]any{"port": "not-a-number"}},
		},
	})

	assert.True(t, errorList.ContainsErrors(), "Service with string port should produce errors")
}

// TestSchemaValidationRule_CustomResourceSkipped covers the rule's boundary: only
// standard Kubernetes resources are validated, so a custom resource passes through
// untouched however malformed it is.
func TestSchemaValidationRule_CustomResourceSkipped(t *testing.T) {
	errorList := runSchemaRule(t, nil, map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata":   map[string]any{"name": "c"},
		"spec":       map[string]any{"secretName": "s", "dnsNames": int64(12345)},
	}, map[string]any{
		"apiVersion": "totally.unknown.io/v1",
		"kind":       "Nonexistent",
		"metadata":   map[string]any{"name": "x"},
		"spec":       map[string]any{"whatever": true},
	})

	assert.False(t, errorList.ContainsErrors(), "resources without a bundled schema must be skipped")
}

func TestSchemaValidationRule_Excluded(t *testing.T) {
	exclude := []pkg.KindRuleExclude{{Kind: "Service", Name: "svc"}}

	errorList := runSchemaRule(t, exclude, map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]any{"name": "svc"},
		"spec": map[string]any{
			"ports": []any{map[string]any{"port": "not-a-number"}},
		},
	})

	assert.False(t, errorList.ContainsErrors(), "excluded resource must not be validated")
}

// TestSchemaValidationRule_BinaryContentNotJudged is the guard for a false positive
// that fired on real modules: a chart passing a value straight into Secret.data —
// the value being expected to arrive already base64-encoded — renders, under the
// values dmt generates from the openapi schema, a payload that is not valid base64.
// The complaint was about dmt's own invention, not about the module, and reads
// especially badly because the decoder cannot even say which field it meant
// ("illegal base64 data at input byte 4").
func TestSchemaValidationRule_BinaryContentNotJudged(t *testing.T) {
	errorList := runSchemaRule(t, nil, map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "passthrough"},
		"type":       "Opaque",
		"data":       map[string]any{"ca.crt": "not-base64-at-all!"},
	})

	assert.Empty(t, errorList.GetErrors(),
		"the content of a binary field is generated by dmt and must not be reported")
}

// TestSchemaValidationRule_BinaryFieldStructureStillJudged is the other half: only
// the content is beyond dmt's reach. The shape around it is still the module's own,
// so a data map that is not a map, a value that is not a string, and a field the API
// does not declare all have to survive the pass that lets the base64 through.
func TestSchemaValidationRule_BinaryFieldStructureStillJudged(t *testing.T) {
	for name, tc := range map[string]struct {
		object map[string]any
		want   string
	}{
		"data is not a map": {
			object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata":   map[string]any{"name": "s"},
				"data":       int64(5),
			},
			want: "cannot restore map",
		},
		"data value is not a string": {
			object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata":   map[string]any{"name": "s"},
				"data":       map[string]any{"key": int64(12345)},
			},
			want: "cannot restore slice",
		},
		"undeclared field alongside a binary one": {
			object: map[string]any{
				"apiVersion":    "v1",
				"kind":          "Secret",
				"metadata":      map[string]any{"name": "s"},
				"data":          map[string]any{"key": "still-not-base64"},
				"immutableTypo": false,
			},
			want: `unknown field "immutableTypo"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			errs := runSchemaRule(t, nil, tc.object).GetErrors()

			require.Len(t, errs, 1)
			assert.Contains(t, errs[0].Text, tc.want)
		})
	}
}
