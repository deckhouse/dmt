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
	"context"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const SchemaValidationRuleName = "schema-validation"

func NewSchemaValidationRule(excludeRules []pkg.KindRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *SchemaValidationRule {
	return &SchemaValidationRule{
		RuleMeta: pkg.RuleMeta{
			Name: SchemaValidationRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(SchemaValidationRuleName),
	}
}

// SchemaValidationRule decodes every rendered manifest into the Go type that
// serves it, strictly: a field of the wrong type and a field the API does not
// declare are both errors. The types come from the k8s.io/api version dmt is
// built against, so what the rule checks against is whatever Kubernetes release
// that is — there is no schema snapshot to keep in sync.
//
// Only standard Kubernetes resources are checked. A custom resource has no Go
// type registered and is skipped, as is any other unregistered kind: the rule
// reports violations, never the absence of a type.
type SchemaValidationRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*SchemaValidationRule)(nil)

func (r *SchemaValidationRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		kind := object.Unstructured.GetKind()

		if !r.Enabled(kind, object.Unstructured.GetName()) {
			continue
		}

		for _, violation := range check(object.Unstructured) {
			r.errorList.WithObjectID(object.Identity()).
				WithFilePath(object.GetPath()).
				Errorf("resource does not match the %s API: %s", kind, violation)
		}
	}
}

// check reads one object into its API type and returns what did not fit.
func check(u unstructured.Unstructured) []string {
	gvk := u.GroupVersionKind()

	// New reports an error for a kind the scheme does not know, which is how a
	// custom resource — anything served by a CRD rather than by the API server
	// itself — is left alone.
	typed, err := scheme.Scheme.New(gvk)
	if err != nil {
		return nil
	}

	if err := decode(u.UnstructuredContent(), typed); err == nil {
		return nil
	}

	// The failure may be about the content of a binary field rather than the shape
	// of the object, and that content is dmt's own invention (see blankBinaryLeaves).
	// Read it again with that content out of the picture, and report only what
	// survives. The second pass needs a fresh object, since the first left this one
	// half-filled, and a copy of the content, since the store is shared with every
	// other rule — both of which are why this is not simply how the first pass works.
	typed, err = scheme.Scheme.New(gvk)
	if err != nil {
		return nil
	}

	clean := runtime.DeepCopyJSON(u.UnstructuredContent())
	blankBinaryLeaves(clean, reflect.TypeOf(typed).Elem())

	if err := decode(clean, typed); err != nil {
		return violations(err)
	}

	return nil
}

// decode strictly reads an object into its API type: a field of the wrong type and
// a field the type does not declare are both errors.
func decode(content map[string]any, typed any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(content, typed, true)
}

// blankBinaryLeaves walks content alongside the shape of t, emptying every string
// that would be read into a []byte field.
//
// Such a field travels as base64, and dmt renders with values generated from the
// module's openapi schema rather than the ones a cluster supplies. A chart that
// passes a value straight into Secret.data — the value being expected to arrive
// already encoded — therefore produces a payload that is not valid base64, and the
// decode fails on dmt's own invention rather than on anything the module got wrong.
//
// The structure is still judged: only strings are emptied, so a data field that is
// not a map, or a map holding a number where the API wants a string, still fails.
func blankBinaryLeaves(value any, t reflect.Type) any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch {
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8:
		if _, ok := value.(string); ok {
			return ""
		}

		return value

	case t.Kind() == reflect.Struct:
		obj, ok := value.(map[string]any)
		if !ok {
			return value
		}

		for i := range t.NumField() {
			field := t.Field(i)

			name, inline := jsonFieldName(field)
			if inline {
				// An embedded struct's own fields sit at this level, so it is walked
				// against the same map rather than a member of it.
				blankBinaryLeaves(obj, field.Type)

				continue
			}

			if v, ok := obj[name]; ok {
				obj[name] = blankBinaryLeaves(v, field.Type)
			}
		}

		return obj

	case t.Kind() == reflect.Map:
		obj, ok := value.(map[string]any)
		if !ok {
			return value
		}

		for k, v := range obj {
			obj[k] = blankBinaryLeaves(v, t.Elem())
		}

		return obj

	case t.Kind() == reflect.Slice || t.Kind() == reflect.Array:
		items, ok := value.([]any)
		if !ok {
			return value
		}

		for i := range items {
			items[i] = blankBinaryLeaves(items[i], t.Elem())
		}

		return items

	default:
		return value
	}
}

// jsonFieldName is the key a struct field is carried under, and whether it is
// embedded at the parent's level instead of under a key of its own.
func jsonFieldName(field reflect.StructField) (string, bool) {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" {
		return field.Name, field.Anonymous
	}

	return name, false
}

// violations flattens a decode failure into one message per problem. Unknown
// fields arrive as a strict-decoding error carrying one entry per field, already
// ordered; anything else is a type mismatch, which aborts the decode and so is
// always a single message.
func violations(err error) []string {
	strict, ok := runtime.AsStrictDecodingError(err)
	if !ok {
		return []string{err.Error()}
	}

	unknown := strict.Errors()
	out := make([]string, 0, len(unknown))

	for _, e := range unknown {
		out = append(out, e.Error())
	}

	return out
}
