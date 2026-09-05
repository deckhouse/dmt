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

// Package defaults generates a module's render values from its openapi schemas:
// it fills every property, honors `not: {required: [...]}` mutual exclusion, and
// picks the fullest x-examples entry. It is dmt's own value generator (the
// packages plugin has an equivalent under values/schema/defaults).
package defaults

import (
	"dario.cat/mergo"
	"github.com/go-openapi/spec"
	"github.com/mohae/deepcopy"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/internal/modules/values/schema/reggen"
)

const (
	dmtDefault      = "x-dmt-default"
	examplesDefault = "x-examples"
	exampleDefault  = "x-example"
	arrayObject     = "array"
	objectKey       = "object"
)

// Generate produces a values map from an openapi schema: it fills every property,
// honoring `not` exclusions and x-examples along the way.
func Generate(root *spec.Schema) (map[string]any, error) {
	return synthesizeProperties(root)
}

func synthesizeProperties(tempNode *spec.Schema) (map[string]any, error) {
	if tempNode == nil {
		return nil, nil
	}

	result := make(map[string]any)
	for key := range tempNode.Properties {
		if err := synthesizeProperty(key, ptr.To(tempNode.Properties[key]), result); err != nil {
			return nil, err
		}
	}

	dropExcluded(tempNode, result)

	return result, nil
}

// dropExcluded honors a `not: {required: [...]}` constraint — the JSON Schema
// idiom deckhouse uses for mutually exclusive fields. dmt fills every property,
// so it can set a whole group a real config never sets together, which a template
// guard then rejects (e.g. openstack's tenantName and tenantID). When every field
// of such a group is present, the first is kept and the rest dropped, so the
// generated config no longer violates the exclusion. Other `not` forms are left
// untouched.
func dropExcluded(node *spec.Schema, result map[string]any) {
	if node.Not == nil || len(node.Not.Required) < 2 {
		return
	}

	for _, name := range node.Not.Required {
		if _, ok := result[name]; !ok {
			return
		}
	}

	for _, name := range node.Not.Required[1:] {
		delete(result, name)
	}
}

func synthesizeProperty(key string, prop *spec.Schema, result map[string]any) error {
	switch {
	case prop.Extensions[dmtDefault] != nil:
		return synthesizeDefault(key, prop, dmtDefault, result)
	case prop.Extensions[exampleDefault] != nil:
		return synthesizeDefault(key, prop, exampleDefault, result)
	case prop.Extensions[examplesDefault] != nil:
		return synthesizeDefault(key, prop, examplesDefault, result)
	case len(prop.Enum) > 0:
		synthesizeEnum(key, prop, result)
	case prop.Type.Contains(objectKey):
		return synthesizeObject(key, prop, result)
	case prop.Default != nil:
		result[key] = prop.Default
	case prop.Type.Contains(arrayObject) && prop.Items != nil && prop.Items.Schema != nil:
		return synthesizeArray(key, prop, result)
	case prop.Type.Contains("integer"):
		result[key] = 123
	case prop.Type.Contains("number"):
		result[key] = 123
	case prop.Type.Contains("boolean"):
		result[key] = true
	case prop.Type.Contains("string"):
		return synthesizeString(key, prop.Pattern, result)
	case len(prop.AllOf) > 0:
		return synthesizeComposite(key, prop, prop.AllOf, result)
	case len(prop.OneOf) > 0:
		return synthesizeComposite(key, prop, prop.OneOf, result)
	case len(prop.AnyOf) > 0:
		return synthesizeComposite(key, prop, prop.AnyOf, result)
	}

	return nil
}

func synthesizeString(key, pattern string, result map[string]any) error {
	if pattern == "" {
		// No pattern in the module's own values schema, so we invent a placeholder.
		// Generate a lowercase kebab-case string (e.g. "abcd-efgh") rather than an
		// arbitrary mixed-case one: such values routinely flow into resource name /
		// namespace / label fields, which downstream CRD schemas constrain to the
		// DNS-1123 label pattern `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`. A mixed-case
		// placeholder fails that pattern and produces spurious schema-validation
		// findings; a lowercase kebab value satisfies it.
		pattern = `^[a-z]{4}-[a-z]{4}$`
	}

	const limit = 8

	r, err := reggen.Generate(pattern, limit)
	if err != nil {
		return err
	}

	result[key] = r

	return nil
}

func synthesizeDefault(key string, prop *spec.Schema, extension string, result map[string]any) error {
	def, ok := prop.Extensions[extension]
	if !ok {
		return nil
	}

	if extension == examplesDefault {
		def = pickExample(def)
		if def == nil {
			return nil
		}
	}

	// An array property is often illustrated with a single element (one
	// toleration, one host) instead of a one-element list. Keep the shape the
	// schema declares: a helper that renders the value with toYaml would
	// otherwise emit a map where the chart appends list items, producing
	// invalid YAML.
	if prop.Type.Contains(arrayObject) {
		def = asList(def)
	}

	ex, ok := def.(map[string]any)
	if !ok {
		result[key] = def
		return nil
	}

	if prop.Type.Contains(objectKey) {
		t, err := synthesizeProperties(prop)
		if err != nil {
			return err
		}

		if err := mergo.Merge(&t, ex, mergo.WithOverride); err != nil {
			return err
		}

		result[key] = t

		return nil
	}

	result[key] = def

	return nil
}

// asList returns v unchanged when it already is a list, and wraps it into a
// one-element list otherwise.
func asList(v any) any {
	switch v.(type) {
	case []any, []map[string]any:
		return v
	default:
		return []any{v}
	}
}

// pickExample chooses which x-examples entry to render. x-examples is a list of
// illustrative configs; dmt renders a single value set per module. For object
// examples it returns the fullest one (see richestExample); for scalar or mixed
// lists it keeps the first entry, as before. A nil or empty list yields nil.
func pickExample(v any) any {
	switch list := v.(type) {
	case []map[string]any:
		if len(list) == 0 {
			return nil
		}

		return richestExample(list)
	case []any:
		if len(list) == 0 {
			return nil
		}

		maps := make([]map[string]any, 0, len(list))
		for _, e := range list {
			if m, ok := e.(map[string]any); ok {
				maps = append(maps, m)
			}
		}

		// Only reorder when every entry is an object; scalar examples have no
		// notion of "fullest", so preserve the historical first-entry behaviour.
		if len(maps) == len(list) {
			return richestExample(maps)
		}

		return list[0]
	default:
		return nil
	}
}

// richestExample returns the example with the most top-level fields. dmt renders
// one value set per module, so taking the first example — often the minimal
// "disabled" case authors list first — both hides conditionally-rendered
// resources and feeds unrealistic values downstream (e.g. https.mode: Disabled
// overriding a CertManager default). The fullest example exercises the most code
// and is the most likely to be internally consistent. Ties keep the earlier one.
func richestExample(examples []map[string]any) map[string]any {
	best := examples[0]
	for _, ex := range examples[1:] {
		if len(ex) > len(best) {
			best = ex
		}
	}

	return best
}

func synthesizeEnum(key string, prop *spec.Schema, result map[string]any) {
	t := prop.Enum[0]
	if prop.Default != nil {
		t = prop.Default
	}

	result[key] = t
}

func synthesizeObject(key string, prop *spec.Schema, result map[string]any) error {
	t, err := synthesizeProperties(prop)
	if err != nil {
		return err
	}

	result[key] = t

	return nil
}

func synthesizeArray(key string, prop *spec.Schema, result map[string]any) error {
	if prop.Items.Schema.Default != nil {
		result[key] = prop.Items.Schema.Default

		return nil
	}

	t := make(map[string]any)
	if err := synthesizeProperty(key, prop.Items.Schema, t); err != nil {
		return err
	}

	result[key] = []any{t[key]}

	return nil
}

// synthesizeComposite fills a value for a oneOf/anyOf/allOf property by merging the
// property with all of its branches' properties and generating from the result.
func synthesizeComposite(key string, prop *spec.Schema, branches []spec.Schema, result map[string]any) error {
	downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
	mergedSchema := mergeSchemas(downwardSchema, branches...)

	if len(mergedSchema.Properties) > 0 {
		t, err := synthesizeProperties(mergedSchema)
		if err != nil {
			return err
		}

		if t != nil {
			result[key] = t
		}

		return nil
	}

	// No object shape to build: pick the first branch with a concrete scalar
	// type (or enum) and generate that, so int-or-string style unions yield a
	// valid scalar rather than an empty object.
	for i := range branches {
		branch := branches[i]
		if len(branch.Enum) > 0 || branch.Type.Contains("string") ||
			branch.Type.Contains("integer") || branch.Type.Contains("number") ||
			branch.Type.Contains("boolean") {
			return synthesizeProperty(key, &branch, result)
		}
	}

	return nil
}

func mergeSchemas(rootSchema *spec.Schema, schemas ...spec.Schema) *spec.Schema {
	if rootSchema == nil {
		rootSchema = &spec.Schema{}
	}

	if rootSchema.Properties == nil {
		rootSchema.Properties = make(map[string]spec.Schema)
	}

	for i := range schemas {
		for key := range schemas[i].Properties {
			rootSchema.Properties[key] = schemas[i].Properties[key]
		}
	}

	return rootSchema
}
