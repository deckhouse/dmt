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
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/require"
)

func Test_parseProperties(t *testing.T) {
	tests := []struct {
		name    string
		schema  *spec.Schema
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "nil schema",
			schema:  nil,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "empty schema",
			schema:  &spec.Schema{},
			want:    map[string]any{},
			wantErr: false,
		},
		{
			name: "schema with simple x-dmt-example",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"exampleKey": {
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"object"},
								Properties: map[string]spec.Schema{
									"bar1": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
									"bar2": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
								},
							},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									DmtDefault: map[string]any{
										"bar1": "example",
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"exampleKey": map[string]any{"bar1": "example", "bar2": "text"}},
			wantErr: false,
		},
		{
			name: "schema with simple x-dmt-example, x-example",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"exampleKey": {
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"object"},
								Properties: map[string]spec.Schema{
									"bar1": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
									"bar2": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
								},
							},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									DmtDefault: map[string]any{
										"bar1": "example",
									},
									ExampleDefault: map[string]any{
										"bar1": "text2",
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"exampleKey": map[string]any{"bar1": "example", "bar2": "text"}},
			wantErr: false,
		},
		{
			name: "schema with simple x-example",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"exampleKey": {
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"object"},
								Properties: map[string]spec.Schema{
									"bar1": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
									"bar2": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
								},
							},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									ExampleDefault: map[string]any{
										"bar1": "text2",
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"exampleKey": map[string]any{"bar1": "text2", "bar2": "text"}},
			wantErr: false,
		},
		{
			name: "schema with simple x-examples",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"exampleKey": {
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"object"},
								Properties: map[string]spec.Schema{
									"bar1": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
									"bar2": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Default: "text",
										},
									},
								},
							},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									ExamplesDefault: []map[string]any{
										{
											"bar1": "text2",
										},
										{
											"bar2": "text2",
										},
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"exampleKey": map[string]any{"bar1": "text2", "bar2": "text"}},
			wantErr: false,
		},
		{
			name: "schema with enum",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"enumKey": {
							SchemaProps: spec.SchemaProps{
								Enum: []any{"enumValue", "enumValue2"},
							},
						},
					},
				},
			},
			want:    map[string]any{"enumKey": "enumValue"},
			wantErr: false,
		},
		{
			name: "schema with object",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"objectKey": {
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"object"},
								Properties: map[string]spec.Schema{
									"nestedKey": {
										SchemaProps: spec.SchemaProps{
											Default: "nestedValue",
										},
									},
									"nestedKey2": {
										SchemaProps: spec.SchemaProps{
											Pattern: "^[a-z]+$",
										},
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"objectKey": map[string]any{"nestedKey": "nestedValue"}},
			wantErr: false,
		},
		{
			name: "schema with array",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"arrayKey": {
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"array"},
								Items: &spec.SchemaOrArray{
									Schema: &spec.Schema{
										SchemaProps: spec.SchemaProps{
											Default: "arrayValue",
										},
									},
									Schemas: []spec.Schema{
										{
											SchemaProps: spec.SchemaProps{
												Default: "arrayValue",
											},
										},
										{
											SchemaProps: spec.SchemaProps{
												Default: "arrayValue2",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"arrayKey": "arrayValue"},
			wantErr: false,
		},
		{
			name: "schema with oneOf",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"oneOfKey": {
							SchemaProps: spec.SchemaProps{
								OneOf: []spec.Schema{
									{
										SchemaProps: spec.SchemaProps{
											Properties: map[string]spec.Schema{
												"oneOfNestedKey": {
													SchemaProps: spec.SchemaProps{
														Default: "oneOfValue",
													},
												},
											},
										},
									},
									{
										SchemaProps: spec.SchemaProps{
											Properties: map[string]spec.Schema{
												"oneOfNestedKey2": {
													SchemaProps: spec.SchemaProps{
														Default: "oneOfValue2",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"oneOfKey": map[string]any{"oneOfNestedKey": "oneOfValue", "oneOfNestedKey2": "oneOfValue2"}},
			wantErr: false,
		},
		{
			name: "schema with anyOf",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"anyOfKey": {
							SchemaProps: spec.SchemaProps{
								AnyOf: []spec.Schema{
									{
										SchemaProps: spec.SchemaProps{
											Properties: map[string]spec.Schema{
												"anyOfNestedKey": {
													SchemaProps: spec.SchemaProps{
														Default: "anyOfValue",
													},
												},
											},
										},
									},
									{
										SchemaProps: spec.SchemaProps{
											Properties: map[string]spec.Schema{
												"anyOfNestedKey2": {
													SchemaProps: spec.SchemaProps{
														Default: "anyOfValue2",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"anyOfKey": map[string]any{"anyOfNestedKey": "anyOfValue", "anyOfNestedKey2": "anyOfValue2"}},
			wantErr: false,
		},
		{
			name: "schema with AllOf",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"allOfKey": {
							SchemaProps: spec.SchemaProps{
								AllOf: []spec.Schema{
									{
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"object"},
											Properties: map[string]spec.Schema{
												"nestedKey1": {
													SchemaProps: spec.SchemaProps{
														Default: "nestedValue",
													},
												},
												"nestedKey2": {
													SchemaProps: spec.SchemaProps{
														Pattern: "^[a-z]+$",
													},
												},
											},
										},
									},
									{
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"object"},
											Properties: map[string]spec.Schema{
												"nestedKey3": {
													SchemaProps: spec.SchemaProps{
														Default: "nestedValue",
													},
												},
												"nestedKey4": {
													SchemaProps: spec.SchemaProps{
														Pattern: "^[a-z]+$",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"allOfKey": map[string]any{"nestedKey1": "nestedValue", "nestedKey3": "nestedValue"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProperties(tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseProperties() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_applyDigests(t *testing.T) {
	tests := []struct {
		name    string
		digests map[string]any
		values  map[string]any
		want    map[string]any
	}{
		{
			name:    "empty digests and values",
			digests: map[string]any{},
			values:  map[string]any{},
			want: map[string]any{
				"global": map[string]any{
					"modulesImages": map[string]any{
						"digests": map[string]any{},
						"registry": map[string]any{
							"base": "registry.example.com/deckhouse",
						},
					},
				},
				"myModule": map[string]any{
					"registry": map[string]any{
						"dockercfg": "ZG9ja2VyY2Zn",
					},
				},
			},
		},
		{
			name: "non-empty digests and values",
			digests: map[string]any{
				"image1": "digest1",
			},
			values: map[string]any{
				"existingKey": "existingValue",
			},
			want: map[string]any{
				"existingKey": "existingValue",
				"global": map[string]any{
					"modulesImages": map[string]any{
						"digests": map[string]any{
							"image1": "digest1",
						},
						"registry": map[string]any{
							"base": "registry.example.com/deckhouse",
						},
					},
				},
				"myModule": map[string]any{
					"registry": map[string]any{
						"dockercfg": "ZG9ja2VyY2Zn",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyDigests("myModule", tt.digests, tt.values)
			require.Equal(t, tt.want, tt.values)
		})
	}
}

func Test_NewOpenAPIValuesGenerator(t *testing.T) {
	schema := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Properties: map[string]spec.Schema{
				"testKey": {
					SchemaProps: spec.SchemaProps{
						Default: "testValue",
					},
				},
			},
		},
	}

	generator := NewOpenAPIValuesGenerator(schema)
	require.NotNil(t, generator)
	require.Equal(t, schema, generator.rootSchema)
}

func Test_OpenAPIValuesGenerator_Do(t *testing.T) {
	tests := []struct {
		name    string
		schema  *spec.Schema
		want    map[string]any
		wantErr bool
	}{
		{
			name: "simple schema with default",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"testKey": {
							SchemaProps: spec.SchemaProps{
								Default: "testValue",
							},
						},
					},
				},
			},
			want:    map[string]any{"testKey": "testValue"},
			wantErr: false,
		},
		{
			name:    "nil schema",
			schema:  nil,
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewOpenAPIValuesGenerator(tt.schema)
			got, err := generator.Do()
			if (err != nil) != tt.wantErr {
				t.Errorf("Do() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_parseString(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		pattern string
		result  map[string]any
		wantErr bool
	}{
		{
			name:    "empty pattern",
			key:     "testKey",
			pattern: "",
			result:  make(map[string]any),
			wantErr: false,
		},
		{
			name:    "custom pattern",
			key:     "testKey",
			pattern: "^[a-z]{3}$",
			result:  make(map[string]any),
			wantErr: false,
		},
		{
			name:    "invalid pattern",
			key:     "testKey",
			pattern: "[invalid",
			result:  make(map[string]any),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseString(tt.key, tt.pattern, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				require.Contains(t, tt.result, tt.key)
				require.NotEmpty(t, tt.result[tt.key])
			}
		})
	}
}

func Test_parseArray(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		prop         *spec.Schema
		result       map[string]any
		wantErr      bool
		expectedType any
	}{
		{
			name: "array with default in items",
			key:  "testArray",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"array"},
					Items: &spec.SchemaOrArray{
						Schema: &spec.Schema{
							SchemaProps: spec.SchemaProps{
								Default: "defaultValue",
							},
						},
					},
				},
			},
			result:       make(map[string]any),
			wantErr:      false,
			expectedType: "defaultValue", // When default is set, it returns the default value, not an array
		},
		{
			name: "array with string items",
			key:  "testArray",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"array"},
					Items: &spec.SchemaOrArray{
						Schema: &spec.Schema{
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"string"},
							},
						},
					},
				},
			},
			result:       make(map[string]any),
			wantErr:      false,
			expectedType: []any{},
		},
		{
			name: "array with object items",
			key:  "testArray",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"array"},
					Items: &spec.SchemaOrArray{
						Schema: &spec.Schema{
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"object"},
								Properties: map[string]spec.Schema{
									"nestedKey": {
										SchemaProps: spec.SchemaProps{
											Default: "nestedValue",
										},
									},
								},
							},
						},
					},
				},
			},
			result:       make(map[string]any),
			wantErr:      false,
			expectedType: []any{},
		},
		{
			name: "array with schemas instead of schema",
			key:  "testArray",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"array"},
					Items: &spec.SchemaOrArray{
						Schema: nil, // Explicitly set to nil to test the schemas path
						Schemas: []spec.Schema{
							{
								SchemaProps: spec.SchemaProps{
									Default: "schemaValue",
								},
							},
						},
					},
				},
			},
			result:       make(map[string]any),
			wantErr:      false,
			expectedType: []any{},
		},
		{
			name: "array with nil Items",
			key:  "testArray",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type:  spec.StringOrArray{"array"},
					Items: nil, // Test the fallback when Items is nil
				},
			},
			result:       make(map[string]any),
			wantErr:      false,
			expectedType: nil, // When Items is nil, no value is set for the key
		},
		{
			name: "array with empty Items (both Schema and Schemas are nil/empty)",
			key:  "testArray",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"array"},
					Items: &spec.SchemaOrArray{
						Schema:  nil,             // Schema is nil
						Schemas: []spec.Schema{}, // Schemas is empty
					},
				},
			},
			result:       make(map[string]any),
			wantErr:      false,
			expectedType: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseArray(tt.key, tt.prop, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArray() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.expectedType == nil {
					// When expectedType is nil, the key should not be present in the result
					require.NotContains(t, tt.result, tt.key)
				} else {
					require.Contains(t, tt.result, tt.key)
					// Check the type of the result
					if expectedStr, ok := tt.expectedType.(string); ok {
						require.Equal(t, expectedStr, tt.result[tt.key])
					} else {
						require.IsType(t, tt.expectedType, tt.result[tt.key])
					}
				}
			}
		})
	}
}

func Test_parseDefault(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		prop       *spec.Schema
		extension  string
		result     map[string]any
		wantErr    bool
		wantResult map[string]any
	}{
		{
			name: "x-dmt-default with simple value",
			key:  "testKey",
			prop: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						DmtDefault: "simpleValue",
					},
				},
			},
			extension: DmtDefault,
			result:    make(map[string]any),
			wantErr:   false,
			wantResult: map[string]any{
				"testKey": "simpleValue",
			},
		},
		{
			name: "x-example with simple value",
			key:  "testKey",
			prop: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						ExampleDefault: "exampleValue",
					},
				},
			},
			extension: ExampleDefault,
			result:    make(map[string]any),
			wantErr:   false,
			wantResult: map[string]any{
				"testKey": "exampleValue",
			},
		},
		{
			name: "x-examples with slice of maps",
			key:  "testKey",
			prop: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						ExamplesDefault: []map[string]any{
							{"key1": "value1"},
							{"key2": "value2"},
						},
					},
				},
			},
			extension: ExamplesDefault,
			result:    make(map[string]any),
			wantErr:   false,
			wantResult: map[string]any{
				"testKey": map[string]any{"key1": "value1"},
			},
		},
		{
			name: "x-examples with slice of any",
			key:  "testKey",
			prop: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						ExamplesDefault: []any{"value1", "value2"},
					},
				},
			},
			extension: ExamplesDefault,
			result:    make(map[string]any),
			wantErr:   false,
			wantResult: map[string]any{
				"testKey": "value1",
			},
		},
		{
			name: "x-examples with empty slice",
			key:  "testKey",
			prop: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						ExamplesDefault: []any{},
					},
				},
			},
			extension:  ExamplesDefault,
			result:     make(map[string]any),
			wantErr:    false,
			wantResult: map[string]any{},
		},
		{
			name: "x-examples with nil value",
			key:  "testKey",
			prop: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						ExamplesDefault: nil,
					},
				},
			},
			extension:  ExamplesDefault,
			result:     make(map[string]any),
			wantErr:    false,
			wantResult: map[string]any{},
		},
		{
			name: "object type with x-dmt-default",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"object"},
					Properties: map[string]spec.Schema{
						"nestedKey": {
							SchemaProps: spec.SchemaProps{
								Default: "nestedValue",
							},
						},
					},
				},
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						DmtDefault: map[string]any{
							"overrideKey": "overrideValue",
						},
					},
				},
			},
			extension: DmtDefault,
			result:    make(map[string]any),
			wantErr:   false,
			wantResult: map[string]any{
				"testKey": map[string]any{
					"nestedKey":   "nestedValue",
					"overrideKey": "overrideValue",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseDefault(tt.key, tt.prop, tt.extension, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDefault() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.wantResult, tt.result)
		})
	}
}

func Test_parseEnum(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		prop   *spec.Schema
		result map[string]any
		want   map[string]any
	}{
		{
			name: "enum with values",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Enum: []any{"value1", "value2", "value3"},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": "value1",
			},
		},
		{
			name: "enum with default value",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Enum:    []any{"value1", "value2", "value3"},
					Default: "value2",
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": "value2",
			},
		},
		{
			name: "empty enum",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Enum: []any{},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseEnum(tt.key, tt.prop, tt.result)
			require.Equal(t, tt.want, tt.result)
		})
	}
}

func Test_parseProperty_edge_cases(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		prop   *spec.Schema
		result map[string]any
		want   map[string]any
	}{
		{
			name:   "nil property",
			key:    "testKey",
			prop:   nil,
			result: make(map[string]any),
			want:   map[string]any{},
		},
		{
			name: "integer type",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"integer"},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": 123,
			},
		},
		{
			name: "number type",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"number"},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": 123,
			},
		},
		{
			name: "boolean type",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"boolean"},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": true,
			},
		},
		{
			name: "string type without pattern",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"string"},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": "aBcDeFgH", // Generated string with default pattern
			},
		},
		{
			name: "string type with pattern",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type:    spec.StringOrArray{"string"},
					Pattern: "^[a-z]{3}$",
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": "abc", // Generated string matching pattern
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseProperty(tt.key, tt.prop, tt.result)
			require.NoError(t, err)

			// For string types, we can't predict the exact value due to regex generation
			// so we just check that the key exists and has a non-empty value
			if tt.prop != nil && tt.prop.Type.Contains("string") {
				require.Contains(t, tt.result, tt.key)
				require.NotEmpty(t, tt.result[tt.key])
			} else {
				require.Equal(t, tt.want, tt.result)
			}
		})
	}
}

func Test_mergeSchemas(t *testing.T) {
	tests := []struct {
		name     string
		root     *spec.Schema
		schemas  []spec.Schema
		expected *spec.Schema
	}{
		{
			name: "merge properties",
			root: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"existing": {
							SchemaProps: spec.SchemaProps{
								Default: "existingValue",
							},
						},
					},
				},
			},
			schemas: []spec.Schema{
				{
					SchemaProps: spec.SchemaProps{
						Properties: map[string]spec.Schema{
							"new1": {
								SchemaProps: spec.SchemaProps{
									Default: "newValue1",
								},
							},
						},
					},
				},
				{
					SchemaProps: spec.SchemaProps{
						Properties: map[string]spec.Schema{
							"new2": {
								SchemaProps: spec.SchemaProps{
									Default: "newValue2",
								},
							},
						},
					},
				},
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"existing": {
							SchemaProps: spec.SchemaProps{
								Default: "existingValue",
							},
						},
						"new1": {
							SchemaProps: spec.SchemaProps{
								Default: "newValue1",
							},
						},
						"new2": {
							SchemaProps: spec.SchemaProps{
								Default: "newValue2",
							},
						},
					},
				},
			},
		},
		{
			name: "merge OneOf, AllOf, AnyOf - existing entries are intentionally dropped",
			root: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					OneOf: []spec.Schema{{}},
					AllOf: []spec.Schema{{}},
					AnyOf: []spec.Schema{{}},
				},
			},
			schemas: []spec.Schema{
				{
					SchemaProps: spec.SchemaProps{
						OneOf: []spec.Schema{{}},
						AllOf: []spec.Schema{{}},
						AnyOf: []spec.Schema{{}},
					},
				},
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{},
					OneOf:      []spec.Schema{{}}, // Only the new one, old ones are cleared
					AllOf:      []spec.Schema{{}}, // Only the new one, old ones are cleared
					AnyOf:      []spec.Schema{{}}, // Only the new one, old ones are cleared
				},
			},
		},
		{
			name: "nil root schema",
			root: nil,
			schemas: []spec.Schema{
				{
					SchemaProps: spec.SchemaProps{
						Properties: map[string]spec.Schema{
							"test": {
								SchemaProps: spec.SchemaProps{
									Default: "testValue",
								},
							},
						},
					},
				},
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"test": {
							SchemaProps: spec.SchemaProps{
								Default: "testValue",
							},
						},
					},
				},
			},
		},
		{
			name: "preserve root schema properties while clearing combinators",
			root: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"rootProp": {
							SchemaProps: spec.SchemaProps{
								Default: "rootValue",
							},
						},
					},
					OneOf: []spec.Schema{{}},
					AllOf: []spec.Schema{{}},
					AnyOf: []spec.Schema{{}},
				},
			},
			schemas: []spec.Schema{
				{
					SchemaProps: spec.SchemaProps{
						Properties: map[string]spec.Schema{
							"newProp": {
								SchemaProps: spec.SchemaProps{
									Default: "newValue",
								},
							},
						},
					},
				},
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"rootProp": {
							SchemaProps: spec.SchemaProps{
								Default: "rootValue",
							},
						},
						"newProp": {
							SchemaProps: spec.SchemaProps{
								Default: "newValue",
							},
						},
					},
					// Combinators are cleared even though properties are preserved
					OneOf: []spec.Schema{},
					AllOf: []spec.Schema{},
					AnyOf: []spec.Schema{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeSchemas(tt.root, tt.schemas...)
			require.Equal(t, tt.expected.Properties, result.Properties)
			require.Len(t, result.OneOf, len(tt.expected.OneOf))
			require.Len(t, result.AllOf, len(tt.expected.AllOf))
			require.Len(t, result.AnyOf, len(tt.expected.AnyOf))
		})
	}
}

func Test_helmFormatModuleImages(t *testing.T) {
	// Create a mock module
	mockModule := &Module{
		name:      "testModule",
		namespace: "testNamespace",
		path:      "/test/path",
	}

	rawValues := map[string]any{
		"existingKey": "existingValue",
	}

	result, err := helmFormatModuleImages(mockModule, rawValues)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check that the result contains expected structure
	require.Contains(t, result, "Chart")
	require.Contains(t, result, "Capabilities")
	require.Contains(t, result, "Release")
	require.Contains(t, result, "Values")

	// Check Release structure
	release, ok := result["Release"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "testModule", release["Name"])
	require.Equal(t, "testNamespace", release["Namespace"])
	require.Equal(t, true, release["IsUpgrade"])
	require.Equal(t, true, release["IsInstall"])
	require.Equal(t, 0, release["Revision"])
	require.Equal(t, "Helm", release["Service"])

	// Check Values structure
	values, ok := result["Values"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, values, "existingKey")
	require.Contains(t, values, "global")
	require.Contains(t, values, "testModule")

	// Check global structure
	global, ok := values["global"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, global, "modulesImages")

	modulesImages, ok := global["modulesImages"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, modulesImages, "digests")
	require.Contains(t, modulesImages, "registry")

	// Check module structure
	module, ok := values["testModule"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, module, "registry")
}

func Test_ComposeValuesFromSchemas(t *testing.T) {
	// Create a temporary directory for the test module
	tempDir := t.TempDir()
	modulePath := filepath.Join(tempDir, "testModule")
	openAPIPath := filepath.Join(modulePath, "openapi")

	// Create the openapi directory
	err := os.MkdirAll(openAPIPath, 0755)
	require.NoError(t, err)

	// Create a mock values.yaml file with a simple schema
	valuesYAML := `type: object
properties:
  moduleKey:
    type: string
    default: "moduleValue"
  nestedObject:
    type: object
    properties:
      nestedKey:
        type: string
        default: "nestedValue"
`
	err = os.WriteFile(filepath.Join(openAPIPath, "values.yaml"), []byte(valuesYAML), 0600)
	require.NoError(t, err)

	// Create a mock config-values.yaml file
	configValuesYAML := `type: object
properties:
  configKey:
    type: string
    default: "configValue"
`
	err = os.WriteFile(filepath.Join(openAPIPath, "config-values.yaml"), []byte(configValuesYAML), 0600)
	require.NoError(t, err)

	// Create a mock module
	mockModule := &Module{
		name:      "testModule",
		namespace: "testNamespace",
		path:      modulePath,
	}

	// Create a global schema
	globalSchema := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type: spec.StringOrArray{"object"},
			Properties: map[string]spec.Schema{
				"globalKey": {
					SchemaProps: spec.SchemaProps{
						Type:    spec.StringOrArray{"string"},
						Default: "globalValue",
					},
				},
			},
		},
	}

	// Test the happy path
	result, err := ComposeValuesFromSchemas(mockModule, globalSchema)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check the structure of the result
	require.Contains(t, result, "Chart")
	require.Contains(t, result, "Capabilities")
	require.Contains(t, result, "Release")
	require.Contains(t, result, "Values")

	// Check Release structure
	release, ok := result["Release"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "testModule", release["Name"])
	require.Equal(t, "testNamespace", release["Namespace"])
	require.Equal(t, true, release["IsUpgrade"])
	require.Equal(t, true, release["IsInstall"])
	require.Equal(t, 0, release["Revision"])
	require.Equal(t, "Helm", release["Service"])

	// Check Values structure
	values, ok := result["Values"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, values, "global")
	require.Contains(t, values, "testModule")

	// Check global values
	global, ok := values["global"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "globalValue", global["globalKey"])

	// Check module values (camelized name)
	module, ok := values["testModule"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "moduleValue", module["moduleKey"])

	// Check nested object in module values
	nestedObject, ok := module["nestedObject"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "nestedValue", nestedObject["nestedKey"])

	// Check global modulesImages structure
	require.Contains(t, global, "modulesImages")
	modulesImages, ok := global["modulesImages"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, modulesImages, "digests")
	require.Contains(t, modulesImages, "registry")

	// Check module registry structure
	require.Contains(t, module, "registry")
	registry, ok := module["registry"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ZG9ja2VyY2Zn", registry["dockercfg"])
}

func Test_ComposeValuesFromSchemas_nil_global_schema(t *testing.T) {
	// Create a temporary directory for the test module
	tempDir := t.TempDir()
	modulePath := filepath.Join(tempDir, "testModule")
	openAPIPath := filepath.Join(modulePath, "openapi")

	// Create the openapi directory
	err := os.MkdirAll(openAPIPath, 0755)
	require.NoError(t, err)

	// Create a mock values.yaml file with a simple schema
	valuesYAML := `type: object
properties:
  moduleKey:
    type: string
    default: "moduleValue"
`
	err = os.WriteFile(filepath.Join(openAPIPath, "values.yaml"), []byte(valuesYAML), 0600)
	require.NoError(t, err)

	// Create a mock config-values.yaml file
	configValuesYAML := `type: object
properties:
  configKey:
    type: string
    default: "configValue"
`
	err = os.WriteFile(filepath.Join(openAPIPath, "config-values.yaml"), []byte(configValuesYAML), 0600)
	require.NoError(t, err)

	// Create a mock module
	mockModule := &Module{
		name:      "testModule",
		namespace: "testNamespace",
		path:      modulePath,
	}

	// Test with nil global schema
	result, err := ComposeValuesFromSchemas(mockModule, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check the structure of the result
	require.Contains(t, result, "Chart")
	require.Contains(t, result, "Capabilities")
	require.Contains(t, result, "Release")
	require.Contains(t, result, "Values")

	// Check Values structure
	values, ok := result["Values"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, values, "global")
	require.Contains(t, values, "testModule")

	// Check global values (should be empty when global schema is nil)
	global, ok := values["global"].(map[string]any)
	require.True(t, ok)
	// Global should only contain modulesImages structure, no custom properties
	require.Contains(t, global, "modulesImages")
	require.NotContains(t, global, "globalKey")

	// Check module values
	module, ok := values["testModule"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "moduleValue", module["moduleKey"])

	// Check global modulesImages structure
	modulesImages, ok := global["modulesImages"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, modulesImages, "digests")
	require.Contains(t, modulesImages, "registry")

	// Check module registry structure
	require.Contains(t, module, "registry")
	registry, ok := module["registry"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ZG9ja2VyY2Zn", registry["dockercfg"])
}

func Test_parseOneOf(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		prop    *spec.Schema
		result  map[string]any
		want    map[string]any
		wantErr bool
	}{
		{
			name: "oneOf with simple properties",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					OneOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop1": {
										SchemaProps: spec.SchemaProps{
											Default: "value1",
										},
									},
								},
							},
						},
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop2": {
										SchemaProps: spec.SchemaProps{
											Default: "value2",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"prop1": "value1",
					"prop2": "value2",
				},
			},
			wantErr: false,
		},
		{
			name: "oneOf with nested objects",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					OneOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"nested": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"object"},
											Properties: map[string]spec.Schema{
												"inner1": {
													SchemaProps: spec.SchemaProps{
														Default: "innerValue1",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"nested": map[string]any{
						"inner1": "innerValue1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "oneOf with empty schemas",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					OneOf: []spec.Schema{},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{},
			},
			wantErr: false,
		},
		{
			name: "oneOf with root schema properties",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"rootProp": {
							SchemaProps: spec.SchemaProps{
								Default: "rootValue",
							},
						},
					},
					OneOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"oneOfProp": {
										SchemaProps: spec.SchemaProps{
											Default: "oneOfValue",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"rootProp":  "rootValue",
					"oneOfProp": "oneOfValue",
				},
			},
			wantErr: false,
		},
		{
			name: "oneOf with conflicting properties (last wins)",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					OneOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"conflict": {
										SchemaProps: spec.SchemaProps{
											Default: "first",
										},
									},
								},
							},
						},
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"conflict": {
										SchemaProps: spec.SchemaProps{
											Default: "second",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"conflict": "second",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseOneOf(tt.key, tt.prop, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOneOf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, tt.result)
		})
	}
}

func Test_parseAnyOf(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		prop    *spec.Schema
		result  map[string]any
		want    map[string]any
		wantErr bool
	}{
		{
			name: "anyOf with simple properties",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AnyOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop1": {
										SchemaProps: spec.SchemaProps{
											Default: "value1",
										},
									},
								},
							},
						},
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop2": {
										SchemaProps: spec.SchemaProps{
											Default: "value2",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"prop1": "value1",
					"prop2": "value2",
				},
			},
			wantErr: false,
		},
		{
			name: "anyOf with arrays",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AnyOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"arrayProp": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"array"},
											Items: &spec.SchemaOrArray{
												Schema: &spec.Schema{
													SchemaProps: spec.SchemaProps{
														Default: "arrayItem",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"arrayProp": "arrayItem",
				},
			},
			wantErr: false,
		},
		{
			name: "anyOf with empty schemas",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AnyOf: []spec.Schema{},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{},
			},
			wantErr: false,
		},
		{
			name: "anyOf with root schema properties",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"rootProp": {
							SchemaProps: spec.SchemaProps{
								Default: "rootValue",
							},
						},
					},
					AnyOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"anyOfProp": {
										SchemaProps: spec.SchemaProps{
											Default: "anyOfValue",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"rootProp":  "rootValue",
					"anyOfProp": "anyOfValue",
				},
			},
			wantErr: false,
		},
		{
			name: "anyOf with enums",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AnyOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"enumProp": {
										SchemaProps: spec.SchemaProps{
											Enum: []any{"option1", "option2"},
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"enumProp": "option1",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseAnyOf(tt.key, tt.prop, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAnyOf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, tt.result)
		})
	}
}

func Test_parseAllOf(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		prop    *spec.Schema
		result  map[string]any
		want    map[string]any
		wantErr bool
	}{
		{
			name: "allOf with simple properties",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AllOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop1": {
										SchemaProps: spec.SchemaProps{
											Default: "value1",
										},
									},
								},
							},
						},
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop2": {
										SchemaProps: spec.SchemaProps{
											Default: "value2",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"prop1": "value1",
					"prop2": "value2",
				},
			},
			wantErr: false,
		},
		{
			name: "allOf with nested objects",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AllOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"nested": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"object"},
											Properties: map[string]spec.Schema{
												"inner1": {
													SchemaProps: spec.SchemaProps{
														Default: "innerValue1",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"nested": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"object"},
											Properties: map[string]spec.Schema{
												"inner2": {
													SchemaProps: spec.SchemaProps{
														Default: "innerValue2",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"nested": map[string]any{
						"inner2": "innerValue2", // Last schema wins for nested objects
					},
				},
			},
			wantErr: false,
		},
		{
			name: "allOf with empty schemas",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AllOf: []spec.Schema{},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{},
			},
			wantErr: false,
		},
		{
			name: "allOf with root schema properties",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"rootProp": {
							SchemaProps: spec.SchemaProps{
								Default: "rootValue",
							},
						},
					},
					AllOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"allOfProp": {
										SchemaProps: spec.SchemaProps{
											Default: "allOfValue",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"rootProp":  "rootValue",
					"allOfProp": "allOfValue",
				},
			},
			wantErr: false,
		},
		{
			name: "allOf with conflicting properties (last wins)",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AllOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"conflict": {
										SchemaProps: spec.SchemaProps{
											Default: "first",
										},
									},
								},
							},
						},
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"conflict": {
										SchemaProps: spec.SchemaProps{
											Default: "second",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"conflict": "second",
				},
			},
			wantErr: false,
		},
		{
			name: "allOf with complex nested structures",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AllOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"complex": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"object"},
											Properties: map[string]spec.Schema{
												"level1": {
													SchemaProps: spec.SchemaProps{
														Type: spec.StringOrArray{"object"},
														Properties: map[string]spec.Schema{
															"level2": {
																SchemaProps: spec.SchemaProps{
																	Default: "deepValue",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"complex": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"object"},
											Properties: map[string]spec.Schema{
												"level1": {
													SchemaProps: spec.SchemaProps{
														Type: spec.StringOrArray{"object"},
														Properties: map[string]spec.Schema{
															"level2b": {
																SchemaProps: spec.SchemaProps{
																	Default: "deepValue2",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"complex": map[string]any{
						"level1": map[string]any{
							"level2b": "deepValue2", // Last schema wins for nested objects
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseAllOf(tt.key, tt.prop, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAllOf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, tt.result)
		})
	}
}

func Test_parseProperty_combinator_edge_cases(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		prop    *spec.Schema
		result  map[string]any
		want    map[string]any
		wantErr bool
	}{
		{
			name: "oneOf with nil property",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					OneOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop": {
										SchemaProps: spec.SchemaProps{
											Default: "value",
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"prop": "value",
				},
			},
			wantErr: false,
		},
		{
			name: "anyOf with empty properties",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AnyOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{},
			},
			wantErr: false,
		},
		{
			name: "allOf with mixed types",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AllOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"stringProp": {
										SchemaProps: spec.SchemaProps{
											Type:    spec.StringOrArray{"string"},
											Pattern: "^[a-z]{3}$",
										},
									},
									"intProp": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"integer"},
										},
									},
									"boolProp": {
										SchemaProps: spec.SchemaProps{
											Type: spec.StringOrArray{"boolean"},
										},
									},
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"stringProp": "abc", // Generated string matching pattern
					"intProp":    123,
					"boolProp":   true,
				},
			},
			wantErr: false,
		},
		{
			name: "oneOf with extensions (should not be processed)",
			key:  "testKey",
			prop: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					OneOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Properties: map[string]spec.Schema{
									"prop": {
										SchemaProps: spec.SchemaProps{
											Default: "value",
										},
									},
								},
							},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									DmtDefault: "extensionValue",
								},
							},
						},
					},
				},
			},
			result: make(map[string]any),
			want: map[string]any{
				"testKey": map[string]any{
					"prop": "value",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseProperty(tt.key, tt.prop, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseProperty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// For string types with patterns, we can't predict the exact value
			// so we just check that the key exists and has a non-empty value
			if tt.prop != nil && len(tt.prop.AllOf) > 0 {
				for _, schema := range tt.prop.AllOf {
					for _, prop := range schema.Properties {
						if !prop.Type.Contains("string") || prop.Pattern == "" {
							continue
						}
						require.Contains(t, tt.result, tt.key)
						keyResult := tt.result[tt.key].(map[string]any)
						require.Contains(t, keyResult, "stringProp")
						require.NotEmpty(t, keyResult["stringProp"])
						return
					}
				}
			}

			require.Equal(t, tt.want, tt.result)
		})
	}
}
