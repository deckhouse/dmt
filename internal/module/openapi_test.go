package module

import (
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
			name: "schema with simple examples",
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
									ExamplesKey: map[string]any{
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
			name: "schema with examples",
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
									ExamplesKey: []any{
										map[string]any{
											"bar1": "example",
										},
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
			name: "schema with array and objects",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"arrayKey": {
							SchemaProps: spec.SchemaProps{
								Type: spec.StringOrArray{"array"},
								Items: &spec.SchemaOrArray{
									Schema: &spec.Schema{
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
									Schemas: []spec.Schema{
										{
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
			want:    map[string]any{"arrayKey": map[string]any{"objectKey": map[string]any{"nestedKey": "nestedValue"}}},
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
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyDigests(tt.digests, tt.values)
			require.Equal(t, tt.want, tt.values)
		})
	}
}
