package values

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"github.com/go-openapi/spec"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/module/schema"
)

const (
	valuesFileName       = "values.yaml"
	configValuesFileName = "config-values.yaml"
)

// readOpenAPIFile reads a single file from the openapi directory, returning nil
// (and no error) when the directory or the file does not exist.
func readOpenAPIFile(openAPIDir, fileName string) ([]byte, error) {
	if openAPIDir == "" {
		return nil, nil
	}

	if _, err := os.Stat(openAPIDir); os.IsNotExist(err) {
		return nil, nil
	}

	path := filepath.Join(openAPIDir, fileName)
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	return data, nil
}

type SchemaType string
type Schemas map[SchemaType]*spec.Schema

const (
	ConfigValuesSchema SchemaType = "config"
	ValuesSchema       SchemaType = "values"
	HelmValuesSchema   SchemaType = "helm"
)

//go:embed global-openapi/config-values.yaml
var globalConfigBytes []byte

//go:embed global-openapi/values.yaml
var globalValuesBytes []byte

// LoadSchemaFromBytes returns spec.Schema object loaded from YAML bytes.
func LoadSchemaFromBytes(openAPIContent []byte) (*spec.Schema, error) {
	s := new(spec.Schema)
	if err := yaml.UnmarshalStrict(openAPIContent, s); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	err := spec.ExpandSchema(s, s, nil)
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

func GetGlobalValues(rootDir string) (*spec.Schema, error) {
	configBytes := globalConfigBytes
	valuesBytes := globalValuesBytes

	if rootDir != "" {
		configBytesT, valuesBytesT, err := readConfigFiles(rootDir)
		if err != nil {
			return nil, err
		}

		log.Info("Using global values", slog.String("directory", rootDir))

		configBytes = configBytesT
		valuesBytes = valuesBytesT
	}

	schemas, err := prepareSchemas(configBytes, valuesBytes)
	if err != nil {
		return nil, err
	}

	if values, ok := schemas[ValuesSchema]; !ok || values == nil {
		return nil, fmt.Errorf("cannot find global values schema")
	}

	return schemas[ValuesSchema], nil
}

func readConfigFiles(rootDir string) ([]byte, []byte, error) {
	configValuesFile := filepath.Join(rootDir, "global-hooks", "openapi", "config-values.yaml")
	valuesFile := filepath.Join(rootDir, "global-hooks", "openapi", "values.yaml")

	configBytes, err := os.ReadFile(configValuesFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read config values file: %w", err)
	}

	valuesBytes, err := os.ReadFile(valuesFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read values file: %w", err)
	}

	return configBytes, valuesBytes, nil
}

func GetModuleValues(modulePath string) (*spec.Schema, error) {
	return GetModuleValuesForValuesFile(modulePath, valuesFileName)
}

// GetModuleValuesForValuesFile is like GetModuleValues but loads the module
// values schema from the given file name inside the module's openapi directory
// (e.g. "values_ce.yaml") instead of the default "values.yaml". The
// "config-values.yaml" schema, when present, is always used as the base.
func GetModuleValuesForValuesFile(modulePath, valuesFile string) (*spec.Schema, error) {
	openAPIPath := filepath.Join(modulePath, "openapi")

	configBytes, err := readOpenAPIFile(openAPIPath, configValuesFileName)
	if err != nil {
		return nil, fmt.Errorf("cannot read openAPI schemas: %w", err)
	}

	valuesBytes, err := readOpenAPIFile(openAPIPath, valuesFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read openAPI schemas: %w", err)
	}

	schemas, err := prepareSchemas(configBytes, valuesBytes)
	if err != nil {
		return nil, err
	}

	if values, ok := schemas[ValuesSchema]; !ok || values == nil {
		return nil, fmt.Errorf("cannot find global values schema")
	}

	return schemas[ValuesSchema], nil
}

func OverrideValues(values, vals *chartutil.Values) error {
	if vals == nil {
		return nil
	}

	v := &chartutil.Values{
		"Values": *vals,
	}

	return mergo.Merge(values, v, mergo.WithOverride)
}
