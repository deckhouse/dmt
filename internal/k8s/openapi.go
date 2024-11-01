package k8s

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-openapi/spec"
	"github.com/mohae/deepcopy"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/internal/valuesvalidation"
)

const (
	ExamplesKey = "x-examples"
	ArrayObject = "array"
	ObjectKey   = "object"
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

func helmFormatModuleImages(m *module.Module, rawValues map[string]any) (chartutil.Values, error) {
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

func GetModulesImagesDigests(modulePath string) (map[string]any, error) {
	var (
		modulesDigests map[string]any
		search         bool
	)

	if fi, err := os.Stat(filepath.Join(filepath.Dir(modulePath), "images_digests.json")); err != nil || fi.Size() == 0 {
		search = true
	}

	var err error
	if search {
		modulesDigests = DefaultImagesDigests
	} else {
		modulesDigests, err = getModulesImagesDigestsFromLocalPath(modulePath)
		if err != nil {
			return nil, err
		}
	}

	return modulesDigests, nil
}

func getModulesImagesDigestsFromLocalPath(modulePath string) (map[string]any, error) {
	var digests map[string]any

	imageDigestsRaw, err := os.ReadFile(filepath.Join(filepath.Dir(modulePath), "images_digests.json"))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(imageDigestsRaw, &digests)
	if err != nil {
		return nil, err
	}

	return digests, nil
}

func ComposeValuesFromSchemas(m *module.Module) (chartutil.Values, error) {
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

//nolint:funlen,gocyclo // complex diff
func parseProperties(tempNode *spec.Schema) (map[string]any, error) {
	result := make(map[string]any)

	if tempNode == nil {
		return nil, nil
	}

	for key := range tempNode.Properties {
		prop := tempNode.Properties[key]
		switch {
		case prop.Extensions[ExamplesKey] != nil:
			examples, ok := prop.Extensions[ExamplesKey].([]any)
			if !ok {
				return result, fmt.Errorf("examples property not an array")
			}
			if len(examples) > 0 {
				result[key] = examples[0]
			}
		case len(prop.Enum) > 0:
			result[key] = prop.Enum
		case prop.Type.Contains(ObjectKey):
			if prop.Default == nil {
				continue
			}
			t, err := parseProperties(&prop)
			if err != nil {
				return nil, err
			}
			result[key] = t
		case prop.Default != nil:
			result[key] = prop.Default
		case prop.Type.Contains(ArrayObject) && prop.Items != nil && prop.Items.Schema != nil:
			switch {
			case prop.Items.Schema.Default != nil:
				result[key] = prop.Items.Schema.Default
			case prop.Items.Schema.Type.Contains(ObjectKey):
				if prop.Items.Schema.Default == nil {
					continue
				}
				t, err := parseProperties(prop.Items.Schema)
				if err != nil {
					return nil, err
				}
				result[key] = t
			default:
				continue
			}
		case prop.AllOf != nil:
			// not implemented
			continue
		case prop.OneOf != nil:
			for i := range prop.OneOf {
				schema := prop.OneOf[i]
				downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
				mergedSchema := mergeSchemas(downwardSchema, schema)
				result[key] = mergedSchema
			}
		case prop.AnyOf != nil:
			for i := range prop.AnyOf {
				schema := prop.AnyOf[i]
				downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
				mergedSchema := mergeSchemas(downwardSchema, schema)
				result[key] = mergedSchema
			}
		default:
			continue
		}
	}

	return result, nil
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
