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
	mod.GetPathMock.Return(t.TempDir()) // no crds/ dir -> only bundled schemas used

	errorList := errors.NewLintRuleErrorsList()
	NewSchemaValidationRule(exclude).ValidateResourceSchemas(mod, errorList)

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

func TestSchemaValidationRule_UnknownKindSkipped(t *testing.T) {
	errorList := runSchemaRule(t, nil, map[string]any{
		"apiVersion": "totally.unknown.io/v1",
		"kind":       "Nonexistent",
		"metadata":   map[string]any{"name": "x"},
		"spec":       map[string]any{"whatever": true},
	})

	assert.False(t, errorList.ContainsErrors(), "resources without a schema must be skipped")
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
