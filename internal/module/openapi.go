package module

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/go-openapi/spec"
	"github.com/mohae/deepcopy"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/valuesvalidation"
)

const (
	ExamplesKey = "x-examples"
	ArrayObject = "array"
	ObjectKey   = "object"
)

const (
	imageDigestfile string = "images_digests.json"
)

// applyDigests if ugly because values now are strongly untyped. We have to rewrite this after adding proper global schema
func applyDigests(digests map[string]any, values any) {
	if values == nil {
		return
	}
	value, ok := values.(map[string]any)["global"]
	if value == nil || !ok {
		return
	}
	value, ok = value.(map[string]any)["modulesImages"]
	if value == nil || !ok {
		return
	}
	value.(map[string]any)["digests"] = digests
}

func helmFormatModuleImages(m *Module, rawValues map[string]any) (chartutil.Values, error) {
	caps := chartutil.DefaultCapabilities
	vers := []string(caps.APIVersions)
	vers = append(vers, "autoscaling.k8s.io/v1/VerticalPodAutoscaler")
	caps.APIVersions = vers

	digests, err := GetModulesImagesDigests(m.GetPath())
	if err != nil {
		return nil, err
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

func GetModulesImagesDigests(modulePath string) (modulesDigests map[string]any, err error) {
	var (
		search bool
	)

	if fi, errs := os.Stat(filepath.Join(filepath.Dir(modulePath), imageDigestfile)); errs != nil || fi.Size() == 0 {
		search = true
	}

	if search {
		return DefaultImagesDigests, nil
	}

	modulesDigests, err = getModulesImagesDigestsFromLocalPath(modulePath)
	if err != nil {
		return nil, err
	}

	return modulesDigests, nil
}

func getModulesImagesDigestsFromLocalPath(modulePath string) (map[string]any, error) {
	var digests map[string]any

	imageDigestsRaw, err := os.ReadFile(filepath.Join(filepath.Dir(modulePath), imageDigestfile))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(imageDigestsRaw, &digests)
	if err != nil {
		return nil, err
	}

	return digests, nil
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
	result := make(map[string]any)

	if tempNode == nil {
		return nil, nil
	}

	for key := range tempNode.Properties {
		prop := tempNode.Properties[key]
		if err := parseProperty(key, &prop, result); err != nil {
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
	case prop.AllOf != nil:
		// not implemented
	case prop.OneOf != nil:
		return parseOneOf(key, prop, result)
	case prop.AnyOf != nil:
		return parseAnyOf(key, prop, result)
	}

	return nil
}

func parseExamples(key string, prop *spec.Schema, result map[string]any) error {
	examples, ok := prop.Extensions[ExamplesKey].([]any)
	if !ok {
		return fmt.Errorf("examples property not an array")
	}
	if len(examples) > 0 {
		result[key] = examples[0]
		if prop.Type.Contains(ObjectKey) {
			t, err := parseProperties(prop)
			if err != nil {
				return err
			}
			if obj, ok := result[key].(map[string]any); ok {
				maps.Copy(t, obj)
				result[key] = t
			}
		}
	}

	return nil
}

func parseEnum(key string, prop *spec.Schema, result map[string]any) {
	if prop.Default != nil {
		result[key] = prop.Default
	} else {
		result[key] = prop.Enum[0]
	}
}

func parseObject(key string, prop *spec.Schema, result map[string]any) error {
	if prop.Default == nil {
		return nil
	}
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
	} else if prop.Items.Schema.Type.Contains(ObjectKey) {
		if prop.Items.Schema.Default == nil {
			return nil
		}
		t, err := parseProperties(prop.Items.Schema)
		if err != nil {
			return err
		}
		result[key] = t
	}

	return nil
}

func parseOneOf(key string, prop *spec.Schema, result map[string]any) error {
	for k := range prop.OneOf {
		schema := prop.OneOf[k]
		downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
		mergedSchema := mergeSchemas(downwardSchema, schema)
		result[key] = mergedSchema
	}

	return nil
}

func parseAnyOf(key string, prop *spec.Schema, result map[string]any) error {
	for k := range prop.AnyOf {
		schema := prop.AnyOf[k]
		downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
		mergedSchema := mergeSchemas(downwardSchema, schema)
		result[key] = mergedSchema
	}

	return nil
}

func mergeSchemas(rootSchema *spec.Schema, schemas ...spec.Schema) *spec.Schema {
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
