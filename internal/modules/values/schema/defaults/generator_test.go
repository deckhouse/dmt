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

package defaults

import (
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/require"
)

func Test_synthesizeProperties(t *testing.T) {
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
									dmtDefault: map[string]any{
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
									dmtDefault: map[string]any{
										"bar1": "example",
									},
									exampleDefault: map[string]any{
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
									exampleDefault: map[string]any{
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
									examplesDefault: []map[string]any{
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
		{
			name: "not-required exclusion keeps the first field and drops the rest",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Not: &spec.Schema{
						SchemaProps: spec.SchemaProps{
							Required: []string{"tenantName", "tenantID"},
						},
					},
					Properties: map[string]spec.Schema{
						"tenantName": {
							SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"string"}, Default: "a"},
						},
						"tenantID": {
							SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"string"}, Default: "b"},
						},
					},
				},
			},
			want:    map[string]any{"tenantName": "a"},
			wantErr: false,
		},
		{
			name: "not-required exclusion is a no-op unless every field is present",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Not: &spec.Schema{
						SchemaProps: spec.SchemaProps{
							Required: []string{"tenantName", "tenantID"},
						},
					},
					Properties: map[string]spec.Schema{
						"tenantName": {
							SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"string"}, Default: "a"},
						},
						// no type, so nothing is generated for it
						"tenantID": {},
					},
				},
			},
			want:    map[string]any{"tenantName": "a"},
			wantErr: false,
		},
		{
			name: "array example given as a single object stays a list",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"tolerations": {
							SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"array"}},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									examplesDefault: []any{
										map[string]any{"key": "key1", "operator": "Equal"},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"tolerations": []any{map[string]any{"key": "key1", "operator": "Equal"}},
			},
			wantErr: false,
		},
		{
			name: "array example given as a bare scalar stays a list",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"hosts": {
							SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"array"}},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									examplesDefault: []any{"example.com"},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"hosts": []any{"example.com"}},
			wantErr: false,
		},
		{
			name: "array example already given as a list is kept as is",
			schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Properties: map[string]spec.Schema{
						"hosts": {
							SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"array"}},
							VendorExtensible: spec.VendorExtensible{
								Extensions: spec.Extensions{
									examplesDefault: []any{
										[]any{"a.example.com", "b.example.com"},
									},
								},
							},
						},
					},
				},
			},
			want:    map[string]any{"hosts": []any{"a.example.com", "b.example.com"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := synthesizeProperties(tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("synthesizeProperties() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}

func Test_pickExample(t *testing.T) {
	disabled := map[string]any{"mode": "Disabled"}
	certManager := map[string]any{
		"mode":        "CertManager",
		"certManager": map[string]any{"clusterIssuerName": "letsencrypt"},
	}

	tests := []struct {
		name string
		in   any
		want any
	}{
		{name: "object examples pick the fullest", in: []any{disabled, certManager}, want: certManager},
		{name: "typed map slice picks the fullest", in: []map[string]any{disabled, certManager}, want: certManager},
		{
			name: "ties keep the earlier example",
			in:   []any{map[string]any{"a": 1}, map[string]any{"b": 2}},
			want: map[string]any{"a": 1},
		},
		{name: "scalar examples keep the first", in: []any{"one", "two"}, want: "one"},
		{name: "mixed examples keep the first", in: []any{"one", disabled}, want: "one"},
		{name: "empty list yields nil", in: []any{}, want: nil},
		{name: "nil yields nil", in: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, pickExample(tt.in))
		})
	}
}
