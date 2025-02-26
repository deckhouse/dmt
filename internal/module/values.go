package module

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation/schema"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
	"gopkg.in/yaml.v3"
)

type SchemaType string
type Schemas map[SchemaType]*spec.Schema

const (
	GlobalSchema       SchemaType = "global"
	ModuleSchema       SchemaType = "module"
	ConfigValuesSchema SchemaType = "config"
	ValuesSchema       SchemaType = "values"
	HelmValuesSchema   SchemaType = "helm"
)

//go:embed global-openapi/config-values.yaml
var globalConfigBytes []byte

//go:embed global-openapi/values.yaml
var globalValuesBytes []byte

// YAMLBytesToJSONDoc is a replacement of swag.YAMLData and YAMLDoc to Unmarshal into interface{}.
// swag.BytesToYAML uses yaml.MapSlice to unmarshal YAML. This type doesn't support map merge of YAML anchors.
func YAMLBytesToJSONDoc(data []byte) (json.RawMessage, error) {
	var yamlObj any
	err := yaml.Unmarshal(data, &yamlObj)
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	doc, err := swag.YAMLToJSON(yamlObj)
	if err != nil {
		return nil, fmt.Errorf("yaml to json: %w", err)
	}

	return doc, nil
}

// LoadSchemaFromBytes returns spec.Schema object loaded from YAML bytes.
func LoadSchemaFromBytes(openAPIContent []byte) (*spec.Schema, error) {
	jsonDoc, err := YAMLBytesToJSONDoc(openAPIContent)
	if err != nil {
		return nil, err
	}

	s := new(spec.Schema)
	if err = json.Unmarshal(jsonDoc, s); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	err = spec.ExpandSchema(s, s, nil)
	if err != nil {
		return nil, fmt.Errorf("expand schema: %w", err)
	}

	return s, nil
}

// prepareSchemas loads schemas for config values, values and helm values.
func prepareSchemas(configBytes, valuesBytes []byte) (Schemas, error) {
	schemas := make(Schemas)
	if len(configBytes) > 0 {
		schemaObj, err := LoadSchemaFromBytes(configBytes)
		if err != nil {
			return nil, fmt.Errorf("load '%s' schema: %w", ConfigValuesSchema, err)
		}
		schemas[ConfigValuesSchema] = schema.TransformSchema(
			schemaObj,
			&schema.AdditionalPropertiesTransformer{},
		)
	}

	if len(valuesBytes) > 0 {
		schemaObj, err := LoadSchemaFromBytes(valuesBytes)
		if err != nil {
			return nil, fmt.Errorf("load '%s' schema: %w", ValuesSchema, err)
		}
		schemas[ValuesSchema] = schema.TransformSchema(
			schemaObj,
			&schema.ExtendTransformer{Parent: schemas[ConfigValuesSchema]},
			&schema.AdditionalPropertiesTransformer{},
		)

		schemas[HelmValuesSchema] = schema.TransformSchema(
			schemaObj,
			// Copy schema object.
			&schema.CopyTransformer{},
			// Transform x-required-for-helm
			&schema.RequiredForHelmTransformer{},
		)
	}

	return schemas, nil
}

func GetGlobalValues() (*spec.Schema, error) {
	schema, err := prepareSchemas(globalConfigBytes, globalValuesBytes)
	if err != nil {
		return nil, err
	}

	if values, ok := schema[ValuesSchema]; !ok || values == nil {
		return nil, fmt.Errorf("cannot find global values schema")
	}

	return schema[ValuesSchema], nil
}

func GetModuleValues(modulePath string) (*spec.Schema, error) {
	openAPIPath := filepath.Join(modulePath, "openapi")
	configBytes, valuesBytes, err := utils.ReadOpenAPIFiles(openAPIPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read openAPI schemas: %w", err)
	}

	schema, err := prepareSchemas(configBytes, valuesBytes)
	if err != nil {
		return nil, err
	}

	if values, ok := schema[ValuesSchema]; !ok || values == nil {
		return nil, fmt.Errorf("cannot find global values schema")
	}

	return schema[ValuesSchema], nil
}
