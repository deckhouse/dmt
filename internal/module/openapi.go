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

package module

import (
	"fmt"

	"dario.cat/mergo"
	"github.com/go-openapi/spec"
	"github.com/mohae/deepcopy"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/internal/module/reggen"
	"github.com/deckhouse/dmt/internal/values"
)

const (
	DmtDefault      = "x-dmt-default"
	ExamplesDefault = "x-examples"
	ExampleDefault  = "x-example"
	ArrayObject     = "array"
	ObjectKey       = "object"
)

func applyDigests(moduleName string, digests, helmValues map[string]any) {
	moduleName = ToLowerCamel(moduleName)
	obj := map[string]any{
		"global": map[string]any{
			"modulesImages": map[string]any{
				"digests": digests,
				"registry": map[string]any{
					"base": "registry.example.com/deckhouse",
				},
			},
		},
		moduleName: map[string]any{
			"registry": map[string]any{
				"dockercfg": "ZG9ja2VyY2Zn",
			},
		},
	}

	_ = mergo.Merge(&helmValues, obj, mergo.WithOverride)
}

func helmFormatModuleImages(m *Module, rawValues map[string]any) (chartutil.Values, error) {
	caps := chartutil.DefaultCapabilities
	vers := []string(caps.APIVersions)
	vers = append(vers, "autoscaling.k8s.io/v1/VerticalPodAutoscaler")
	caps.APIVersions = vers

	digests := map[string]any{
		"common": map[string]any{
			"init":      "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc",
			"container": "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc",
		},
		"prompp": map[string]any{
			"prompp": "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc",
		},
		"module": map[string]any{
			"container": "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc",
		},
		"controlPlaneManager": map[string]any{
			"kubeApiserver":         "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc",
			"kubeControllerManager": "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc",
			"kubeScheduler":         "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc",
		},
	}

	applyDigests(m.GetName(), digests, rawValues)
	top := map[string]any{
		"Chart":        m.GetMetadata(),
		"Capabilities": caps,
		"Release": map[string]any{
			"Name":      m.GetName(),
			"Namespace": m.GetNamespace(),
			"IsUpgrade": true,
			"IsInstall": true,
			"Revision":  0,
			"Service":   "Helm",
		},
		"Values": rawValues,
	}

	return top, nil
}

func ComposeValuesFromSchemas(m *Module, globalSchema *spec.Schema) (chartutil.Values, error) {
	if globalSchema == nil {
		globalSchema = &spec.Schema{}
	}

	moduleValues, err := values.GetModuleValues(m.GetPath())
	if err != nil {
		return nil, fmt.Errorf("cannot find openapi values schema for module %q: %w", m.GetName(), err)
	}

	moduleSchema := *moduleValues
	moduleSchema.Default = make(map[string]any)

	camelizedModuleName := ToLowerCamel(m.GetName())
	combinedSchema := spec.Schema{}
	combinedSchema.Properties = map[string]spec.Schema{camelizedModuleName: moduleSchema, "global": *globalSchema}

	rawValues, err := NewOpenAPIValuesGenerator(&combinedSchema).Do()
	if err != nil {
		return nil, fmt.Errorf("generate values: %w", err)
	}

	return helmFormatModuleImages(m, rawValues)
}

type OpenAPIValuesGenerator struct {
	rootSchema *spec.Schema
}

func NewOpenAPIValuesGenerator(schema *spec.Schema) *OpenAPIValuesGenerator {
	return &OpenAPIValuesGenerator{
		rootSchema: schema,
	}
}

func (g *OpenAPIValuesGenerator) Do() (map[string]any, error) {
	return parseProperties(g.rootSchema)
}

func parseProperties(tempNode *spec.Schema) (map[string]any, error) {
	if tempNode == nil {
		return nil, nil
	}

	result := make(map[string]any)
	for key := range tempNode.Properties {
		if err := parseProperty(key, ptr.To(tempNode.Properties[key]), result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func parseProperty(key string, prop *spec.Schema, result map[string]any) error {
	// Check if prop is nil to avoid panic
	if prop == nil {
		return nil
	}

	switch {
	case prop.Extensions[DmtDefault] != nil:
		return parseDefault(key, prop, DmtDefault, result)
	case prop.Extensions[ExampleDefault] != nil:
		return parseDefault(key, prop, ExampleDefault, result)
	case prop.Extensions[ExamplesDefault] != nil:
		return parseDefault(key, prop, ExamplesDefault, result)
	case len(prop.Enum) > 0:
		parseEnum(key, prop, result)
	case prop.Type.Contains(ObjectKey):
		return parseObject(key, prop, result)
	case prop.Default != nil:
		result[key] = prop.Default
	case prop.Type.Contains(ArrayObject) && prop.Items != nil && prop.Items.Schema != nil:
		return parseArray(key, prop, result)
	case prop.Type.Contains("integer"):
		result[key] = 123
	case prop.Type.Contains("number"):
		result[key] = 123
	case prop.Type.Contains("boolean"):
		result[key] = true
	case prop.Type.Contains("string"):
		return parseString(key, prop.Pattern, result)
	case len(prop.AllOf) > 0:
		return parseAllOf(key, prop, result)
	case len(prop.OneOf) > 0:
		return parseOneOf(key, prop, result)
	case len(prop.AnyOf) > 0:
		return parseAnyOf(key, prop, result)
	}

	return nil
}

func parseString(key, pattern string, result map[string]any) error {
	if pattern == "" {
		pattern = `^[a-zA-Z0-9]{8}$`
	}
	const limit = 8
	r, err := reggen.Generate(pattern, limit)
	if err != nil {
		return err
	}
	result[key] = r

	return nil
}

func parseDefault(key string, prop *spec.Schema, extension string, result map[string]any) error {
	def, ok := prop.Extensions[extension]
	if !ok {
		return nil
	}
	if extension == ExamplesDefault {
		if def == nil {
			return nil
		}
		slice, isSlice := def.([]any)
		if isSlice {
			if len(slice) == 0 {
				return nil
			}
			def = slice[0]
		} else {
			mapSlice, isMapSlice := def.([]map[string]any)
			if isMapSlice {
				if len(mapSlice) == 0 {
					return nil
				}
				def = mapSlice[0]
			} else {
				// Skip non-slice and non-map-slice default values as they are not supported in this context
				return nil
			}
		}
	}
	ex, ok := def.(map[string]any)
	if !ok {
		result[key] = def
		return nil
	}
	if prop.Type.Contains(ObjectKey) {
		t, err := parseProperties(prop)
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

func parseEnum(key string, prop *spec.Schema, result map[string]any) {
	if len(prop.Enum) == 0 {
		// Return empty value if enum is empty
		result[key] = nil
		return
	}

	t := prop.Enum[0]
	if prop.Default != nil {
		t = prop.Default
	}
	result[key] = t
}

func parseObject(key string, prop *spec.Schema, result map[string]any) error {
	t, err := parseProperties(prop)
	if err != nil {
		return err
	}
	result[key] = t

	return nil
}

func parseArray(key string, prop *spec.Schema, result map[string]any) error {
	// Check if prop is nil to avoid panic
	if prop == nil {
		result[key] = []any{}
		return nil
	}

	if prop.Items == nil {
		result[key] = []any{}
		return nil
	}
	if prop.Items.Schema != nil && prop.Items.Schema.Default != nil {
		result[key] = prop.Items.Schema.Default
		return nil
	}

	element := prop.Items.Schema
	if element == nil && len(prop.Items.Schemas) > 0 {
		element = &prop.Items.Schemas[0]
	}
	if element == nil {
		result[key] = []any{}
		return nil
	}

	// Use existing parseProperty logic by creating a temporary map with unique key
	tempResult := make(map[string]any)
	if err := parseProperty("_dmt_array_element_", element, tempResult); err != nil {
		return err
	}

	// Extract the parsed value from the temporary map
	var elementValue any
	if val, exists := tempResult["_dmt_array_element_"]; exists {
		elementValue = val
	}

	result[key] = []any{elementValue}
	return nil
}

func parseOneOf(key string, prop *spec.Schema, result map[string]any) error {
	downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
	mergedSchema := mergeSchemas(downwardSchema, prop.OneOf...)

	t, err := parseProperties(mergedSchema)
	if err != nil {
		return err
	}

	if t != nil {
		result[key] = t
	}

	return nil
}

func parseAnyOf(key string, prop *spec.Schema, result map[string]any) error {
	downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
	mergedSchema := mergeSchemas(downwardSchema, prop.AnyOf...)

	t, err := parseProperties(mergedSchema)
	if err != nil {
		return err
	}

	if t != nil {
		result[key] = t
	}

	return nil
}

func parseAllOf(key string, prop *spec.Schema, result map[string]any) error {
	downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
	mergedSchema := mergeSchemas(downwardSchema, prop.AllOf...)

	t, err := parseProperties(mergedSchema)
	if err != nil {
		return err
	}

	if t != nil {
		result[key] = t
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

	// Clear the combined fields at the beginning
	rootSchema.OneOf = nil
	rootSchema.AllOf = nil
	rootSchema.AnyOf = nil

	for i := range schemas {
		schema := schemas[i]
		// Merge properties
		for key := range schema.Properties {
			rootSchema.Properties[key] = schema.Properties[key]
		}
		// Append OneOf, AllOf, AnyOf instead of overwriting
		if len(schema.OneOf) > 0 {
			rootSchema.OneOf = append(rootSchema.OneOf, schema.OneOf...)
		}
		if len(schema.AllOf) > 0 {
			rootSchema.AllOf = append(rootSchema.AllOf, schema.AllOf...)
		}
		if len(schema.AnyOf) > 0 {
			rootSchema.AnyOf = append(rootSchema.AnyOf, schema.AnyOf...)
		}
	}

	return rootSchema
}
