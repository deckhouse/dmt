package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestDeckhouseCRDsRule(t *testing.T) {
	tests := []struct {
		name       string
		moduleName string
		content    string
		wantErrors []string
	}{
		{
			name:       "valid CRD",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: nil,
		},
		{
			name:       "invalid API version",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"`},
		},
		{
			name:       "missing module label",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD should contain "module = test-module" label`},
		},
		{
			name:       "wrong module label",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: wrong-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD should contain "module = test-module" label, but got "module = wrong-module"`},
		},
		{
			name:       "excluded CRD name",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: excluded.deckhouse.io
  labels:
    heritage: deckhouse
    module: wrong-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: nil,
		},
		{
			name:       "CRD with deprecated key",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            deprecatedField:
              type: string
              deprecated: true
              description: This field is deprecated`,
			wantErrors: []string{`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.deprecatedField", use "x-doc-deprecated: true" instead`},
		},
		{
			name:       "CRD with x-doc-deprecated key (should not error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            deprecatedField:
              type: string
              x-doc-deprecated: true
              description: This field is deprecated`,
			wantErrors: nil,
		},
		{
			name:       "CRD with deprecated: false (should error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            activeField:
              type: string
              deprecated: false
              description: This field is active and not deprecated`,
			wantErrors: []string{`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.activeField", use "x-doc-deprecated: true" instead`},
		},
		{
			name:       "CRD with deprecated: true (should error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            deprecatedField:
              type: string
              deprecated: true
              description: This field is deprecated`,
			wantErrors: []string{`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.deprecatedField", use "x-doc-deprecated: true" instead`},
		},
		{
			name:       "CRD with deprecated in version (should not error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      deprecated: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            activeField:
              type: string
              description: This field is active`,
			wantErrors: nil,
		},
		{
			name:       "CRD with deprecated in metadata (should not error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
  deprecated: true
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            activeField:
              type: string
              description: This field is active`,
			wantErrors: nil,
		},
		{
			name:       "CRD with deprecated in nested properties (should error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            nestedObject:
              type: object
              properties:
                nestedField:
                  type: string
                  deprecated: true
                  description: This nested field is deprecated`,
			wantErrors: []string{`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.nestedObject.properties.nestedField", use "x-doc-deprecated: true" instead`},
		},
		{
			name:       "CRD with deprecated in array item schema (should error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            listField:
              type: array
              items:
                type: object
                properties:
                  arrayItemField:
                    type: string
                    deprecated: true
                    description: This field in array item is deprecated`,
			wantErrors: []string{`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.listField.items.properties.arrayItemField", use "x-doc-deprecated: true" instead`},
		},
		{
			name:       "CRD with multiple versions - deprecated in later version (should error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1alpha1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
          properties:
            activeField:
              type: string
              description: This field is active
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            deprecatedField:
              type: string
              deprecated: true
              description: This field is deprecated in v1`,
			wantErrors: []string{`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.deprecatedField", use "x-doc-deprecated: true" instead`},
		},
		{
			name:       "CRD with multiple versions - deprecated in first version only (should error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1alpha1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
          properties:
            deprecatedField:
              type: string
              deprecated: true
              description: This field is deprecated in v1alpha1
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            activeField:
              type: string
              description: This field is active in v1`,
			wantErrors: []string{`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.deprecatedField", use "x-doc-deprecated: true" instead`},
		},
		{
			name:       "CRD with multiple versions - deprecated in both versions (should error)",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1alpha1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
          properties:
            deprecatedField1:
              type: string
              deprecated: true
              description: This field is deprecated in v1alpha1
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            deprecatedField2:
              type: string
              deprecated: true
              description: This field is deprecated in v1`,
			wantErrors: []string{
				`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.deprecatedField1", use "x-doc-deprecated: true" instead`,
				`CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.deprecatedField2", use "x-doc-deprecated: true" instead`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, cleanup := createTempFile(t, tt.content)
			defer cleanup()

			cfg := &config.OpenAPISettings{}
			if tt.name == "excluded CRD name" {
				cfg.OpenAPIExcludeRules.CRDNamesExcludes = []string{"excluded.deckhouse.io"}
			}

			rule := NewDeckhouseCRDsRule(cfg, "test")
			errorList := errors.NewLintRuleErrorsList()
			rule.Run(tt.moduleName, filePath, errorList)

			errs := errorList.GetErrors()
			if tt.wantErrors == nil {
				assert.Empty(t, errs)
			} else {
				assert.Len(t, errs, len(tt.wantErrors))
				for i, err := range errs {
					assert.Contains(t, err.Text, tt.wantErrors[i])
				}
			}
		})
	}
}

func TestAggregateVersionProperties(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expected map[string]any
	}{
		{
			name: "single version with properties",
			data: map[string]any{
				"spec": map[string]any{
					"versions": []any{
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"properties": map[string]any{
										"field1": map[string]any{"type": "string"},
										"field2": map[string]any{"type": "integer"},
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"field1": map[string]any{"type": "string"},
				"field2": map[string]any{"type": "integer"},
			},
		},
		{
			name: "multiple versions with different properties",
			data: map[string]any{
				"spec": map[string]any{
					"versions": []any{
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"properties": map[string]any{
										"field1": map[string]any{"type": "string"},
									},
								},
							},
						},
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"properties": map[string]any{
										"field2": map[string]any{"type": "integer"},
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"field1": map[string]any{"type": "string"},
				"field2": map[string]any{"type": "integer"},
			},
		},
		{
			name: "version without properties",
			data: map[string]any{
				"spec": map[string]any{
					"versions": []any{
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "invalid spec structure",
			data: map[string]any{
				"spec": "not a map",
			},
			expected: nil,
		},
		{
			name: "missing versions",
			data: map[string]any{
				"spec": map[string]any{},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateVersionProperties(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeepMergeProperties(t *testing.T) {
	tests := []struct {
		name     string
		target   map[string]any
		source   map[string]any
		expected map[string]any
	}{
		{
			name: "merge new keys",
			target: map[string]any{
				"field1": map[string]any{"type": "string"},
			},
			source: map[string]any{
				"field2": map[string]any{"type": "integer"},
			},
			expected: map[string]any{
				"field1": map[string]any{"type": "string"},
				"field2": map[string]any{"type": "integer"},
			},
		},
		{
			name: "deep merge nested maps",
			target: map[string]any{
				"nested": map[string]any{
					"field1": map[string]any{"type": "string"},
				},
			},
			source: map[string]any{
				"nested": map[string]any{
					"field2": map[string]any{"type": "integer"},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"field1": map[string]any{"type": "string"},
					"field2": map[string]any{"type": "integer"},
				},
			},
		},
		{
			name: "override non-map values",
			target: map[string]any{
				"field1": "old value",
				"field2": map[string]any{"type": "string"},
			},
			source: map[string]any{
				"field1": "new value",
				"field2": map[string]any{"type": "integer"},
			},
			expected: map[string]any{
				"field1": "new value",
				"field2": map[string]any{"type": "integer"},
			},
		},
		{
			name: "deep merge multiple levels",
			target: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"field1": map[string]any{"type": "string"},
					},
				},
			},
			source: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"field2": map[string]any{"type": "integer"},
					},
				},
			},
			expected: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"field1": map[string]any{"type": "string"},
						"field2": map[string]any{"type": "integer"},
					},
				},
			},
		},
		{
			name: "handle type conflicts - prefer source",
			target: map[string]any{
				"field1": map[string]any{"type": "string"},
			},
			source: map[string]any{
				"field1": "not a map",
			},
			expected: map[string]any{
				"field1": "not a map",
			},
		},
		{
			name:   "empty source",
			target: map[string]any{"field1": "value1"},
			source: map[string]any{},
			expected: map[string]any{
				"field1": "value1",
			},
		},
		{
			name:   "empty target",
			target: map[string]any{},
			source: map[string]any{"field1": "value1"},
			expected: map[string]any{
				"field1": "value1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of target to avoid modifying the original
			targetCopy := make(map[string]any)
			for k, v := range tt.target {
				targetCopy[k] = v
			}

			deepMergeProperties(targetCopy, tt.source)
			assert.Equal(t, tt.expected, targetCopy)
		})
	}
}

func TestAggregateVersionPropertiesWithDeepMerge(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expected map[string]any
	}{
		{
			name: "multiple versions with overlapping properties - deep merge",
			data: map[string]any{
				"spec": map[string]any{
					"versions": []any{
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"properties": map[string]any{
										"nested": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"field1": map[string]any{"type": "string"},
											},
										},
									},
								},
							},
						},
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"properties": map[string]any{
										"nested": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"field2": map[string]any{"type": "integer"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"field1": map[string]any{"type": "string"},
						"field2": map[string]any{"type": "integer"},
					},
				},
			},
		},
		{
			name: "multiple versions with property type changes",
			data: map[string]any{
				"spec": map[string]any{
					"versions": []any{
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"properties": map[string]any{
										"field1": map[string]any{"type": "string"},
									},
								},
							},
						},
						map[string]any{
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"properties": map[string]any{
										"field1": map[string]any{"type": "integer"},
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"field1": map[string]any{"type": "integer"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateVersionProperties(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}
