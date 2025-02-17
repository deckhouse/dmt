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
	"strings"

	"dario.cat/mergo"
	"github.com/go-openapi/spec"
	"github.com/mohae/deepcopy"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/internal/module/reggen"
	"github.com/deckhouse/dmt/internal/valuesvalidation"
)

const (
	ExamplesKey = "x-examples"
	ArrayObject = "array"
	ObjectKey   = "object"
)

func applyDigests(digests, values map[string]any) {
	obj := map[string]any{
		"global": map[string]any{
			"modulesImages": map[string]any{
				"digests": digests,
				"registry": map[string]any{
					"base": "registry.example.com/deckhouse",
				},
			},
		},
	}

	_ = mergo.Merge(&values, obj, mergo.WithOverride)
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

	applyDigests(digests, rawValues)
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

func ComposeValuesFromSchemas(m *Module) (chartutil.Values, error) {
	valueValidator, err := valuesvalidation.NewValuesValidator(m.GetName(), m.GetPath())
	if err != nil {
		return nil, fmt.Errorf("schemas load: %w", err)
	}

	if valueValidator == nil {
		return nil, nil
	}

	camelizedModuleName := ToLowerCamel(m.GetName())

	schema, ok := valueValidator.ModuleSchemaStorages[m.GetName()]
	if !ok || schema.Schemas == nil {
		return nil, nil
	}

	values, ok := valueValidator.ModuleSchemaStorages[m.GetName()].Schemas["values"]
	if values == nil || !ok {
		return nil, fmt.Errorf("cannot find openapi values schema for module %s", m.GetName())
	}

	moduleSchema := *values
	moduleSchema.Default = make(map[string]any)

	values, ok = valueValidator.GlobalSchemaStorage.Schemas["values"]
	var globalSchema spec.Schema
	if ok && values != nil {
		globalSchema = *values
	}
	globalSchema.Default = make(map[string]any)

	combinedSchema := spec.Schema{}
	combinedSchema.Properties = map[string]spec.Schema{camelizedModuleName: moduleSchema, "global": globalSchema}

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
	switch {
	case prop.Extensions[ExamplesKey] != nil:
		return parseExamples(key, prop, result)
	case len(prop.Enum) > 0:
		parseEnum(key, prop, result)
	case prop.Type.Contains(ObjectKey):
		return parseObject(key, prop, result)
	case prop.Default != nil:
		result[key] = prop.Default
	case prop.Type.Contains(ArrayObject) && prop.Items != nil && prop.Items.Schema != nil:
		return parseArray(key, prop, result)
	case prop.Type.Contains("string"):
		return parseString(key, prop.Pattern, result)
	case prop.Type.Contains("integer"):
		result[key] = 123
	case prop.Type.Contains("number"):
		result[key] = 123
	case prop.Type.Contains("boolean"):
		result[key] = true
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
	// ignore cniSecretData key
	if key == "cniSecretData" {
		return nil
	}

	if key == "name" {
		result[key] = "name"
		return nil
	}
	const limit = 8
	if strings.Contains(key, "CPU") {
		result[key] = "100m"
		return nil
	}
	if strings.Contains(key, "Memory") {
		result[key] = "128Mi"
		return nil
	}
	if pattern != "" {
		result[key] = "string"
		r, err := reggen.Generate(pattern, limit)
		if err != nil {
			return err
		}
		result[key] = r
	} else {
		const pattern = "[a-zA-Z0-9]{8}"
		result[key] = "string"
		r, err := reggen.Generate(pattern, limit)
		if err != nil {
			return err
		}
		result[key] = r
	}

	return nil
}

func parseExamples(key string, prop *spec.Schema, result map[string]any) error {
	var example any

	switch conv := prop.Extensions[ExamplesKey].(type) {
	case []any:
		example = conv[0]
	case map[string]any:
		example = conv
	}

	if example != nil {
		ex, ok := example.(map[string]any)
		if !ok {
			result[key] = example
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

		result[key] = example
	}

	return nil
}

func parseEnum(key string, prop *spec.Schema, result map[string]any) {
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
	if prop.Items.Schema.Default != nil {
		result[key] = prop.Items.Schema.Default

		return nil
	}

	element := prop.Items.Schema
	if element == nil && len(prop.Items.Schemas) > 0 {
		element = &prop.Items.Schemas[0]
	}

	t := make(map[string]any)
	err := parseProperty(key, element, t)
	if err != nil {
		return err
	}

	result[key] = []any{t[key]}

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

	rootSchema.OneOf = nil
	rootSchema.AllOf = nil
	rootSchema.AnyOf = nil

	for i := range schemas {
		schema := schemas[i]
		for key := range schema.Properties {
			rootSchema.Properties[key] = schema.Properties[key]
		}
		rootSchema.OneOf = schema.OneOf
		rootSchema.AllOf = schema.AllOf
		rootSchema.AnyOf = schema.AnyOf
	}

	return rootSchema
}
