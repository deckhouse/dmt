/*
Copyright The Helm Authors.

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

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"key": "value"},
			expected: "key: value",
		},
		{
			name:     "nested map",
			input:    map[string]any{"outer": map[string]any{"inner": "value"}},
			expected: "outer:\n  inner: value",
		},
		{
			name:     "array",
			input:    []string{"item1", "item2"},
			expected: "- item1\n- item2",
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toYAML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToYAMLPretty(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"key": "value"},
			expected: "key: value",
		},
		{
			name:     "nested map",
			input:    map[string]any{"outer": map[string]any{"inner": "value"}},
			expected: "outer:\n  inner: value",
		},
		{
			name:     "array",
			input:    []string{"item1", "item2"},
			expected: "- item1\n- item2",
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toYAMLPretty(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:     "simple map",
			input:    "key: value",
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "nested map",
			input:    "outer:\n  inner: value",
			expected: map[string]any{"outer": map[string]any{"inner": "value"}},
		},
		{
			name:     "invalid yaml",
			input:    "invalid: : yaml:",
			expected: map[string]any{"Error": "error converting YAML to JSON: yaml: mapping values are not allowed in this context"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromYAML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromYAMLArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []any
	}{
		{
			name:     "string array",
			input:    "- item1\n- item2",
			expected: []any{"item1", "item2"},
		},
		{
			name:     "nested array",
			input:    "- - nested1\n  - nested2\n- item2",
			expected: []any{[]any{"nested1", "nested2"}, "item2"},
		},
		{
			name:     "invalid yaml array",
			input:    "invalid: yaml array",
			expected: []any{"error unmarshaling JSON: json: cannot unmarshal object into Go value of type []interface {}"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromYAMLArray(tt.input)
			// For the error case, we just check if there's an error string
			if len(tt.expected) == 1 && tt.name == "invalid yaml array" {
				assert.Len(t, result, 1)
				assert.Contains(t, result[0].(string), "error")
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestToTOML(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name: "simple map",
			input: map[string]any{
				"key": "value",
			},
			expected: "key = \"value\"\n",
		},
		{
			name: "nested map",
			input: map[string]any{
				"outer": map[string]any{
					"inner": "value",
				},
			},
			expected: "[outer]\n  inner = \"value\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toTOML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromTOML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:     "simple key-value",
			input:    "key = \"value\"",
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "nested map",
			input:    "[outer]\ninner = \"value\"",
			expected: map[string]any{"outer": map[string]any{"inner": "value"}},
		},
		{
			name:     "invalid toml",
			input:    "invalid = toml =",
			expected: map[string]any{"Error": "toml: line 1 (last key \"invalid\"): expected value but found \"toml\" instead"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromTOML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name: "simple map",
			input: map[string]any{
				"key": "value",
			},
			expected: `{"key":"value"}`,
		},
		{
			name: "nested map",
			input: map[string]any{
				"outer": map[string]any{
					"inner": "value",
				},
			},
			expected: `{"outer":{"inner":"value"}}`,
		},
		{
			name:     "array",
			input:    []string{"item1", "item2"},
			expected: `["item1","item2"]`,
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:     "simple map",
			input:    `{"key":"value"}`,
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "nested map",
			input:    `{"outer":{"inner":"value"}}`,
			expected: map[string]any{"outer": map[string]any{"inner": "value"}},
		},
		{
			name:     "invalid json",
			input:    `{"key":"value"`,
			expected: map[string]any{"Error": "unexpected end of JSON input"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]any{"Error": "unexpected end of JSON input"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromJSONArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []any
	}{
		{
			name:     "string array",
			input:    `["item1","item2"]`,
			expected: []any{"item1", "item2"},
		},
		{
			name:     "nested array",
			input:    `[["nested1","nested2"],"item2"]`,
			expected: []any{[]any{"nested1", "nested2"}, "item2"},
		},
		{
			name:     "invalid json array",
			input:    `["item1","item2"`,
			expected: []any{"unexpected end of JSON input"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []any{"unexpected end of JSON input"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromJSONArray(tt.input)
			if len(tt.expected) == 1 && (tt.name == "invalid json array" || tt.name == "empty string") {
				assert.Len(t, result, 1)
				assert.Equal(t, tt.expected[0], result[0])
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFuncMap(t *testing.T) {
	fm := funcMap()

	// Check that certain functions exist
	expectedFuncs := []string{
		"toToml", "fromToml", "toYaml", "toYamlPretty",
		"fromYaml", "fromYamlArray", "toJson",
		"fromJson", "fromJsonArray", "include", "tpl",
		"required", "lookup", "b64dec", "b32dec",
	}

	for _, funcName := range expectedFuncs {
		if _, ok := fm[funcName]; !ok {
			t.Errorf("Expected function %q not found in funcMap", funcName)
		}
	}

	// Check that env and expandenv are removed
	removedFuncs := []string{"env", "expandenv"}
	for _, funcName := range removedFuncs {
		if _, ok := fm[funcName]; ok {
			t.Errorf("Function %q should be removed but is present in funcMap", funcName)
		}
	}
}

// Test that placeholder functions return expected values
func TestPlaceholderFunctions(t *testing.T) {
	fm := funcMap()

	// Test placeholder functions
	includeFn, ok := fm["include"].(func(string, any) string)
	if !ok {
		t.Fatalf("include function has wrong type")
	}
	assert.Equal(t, "not implemented", includeFn("", nil))

	tplFn, ok := fm["tpl"].(func(string, any) any)
	if !ok {
		t.Fatalf("tpl function has wrong type")
	}
	assert.Equal(t, "not implemented", tplFn("", nil))

	requiredFn, ok := fm["required"].(func(string, any) (any, error))
	if !ok {
		t.Fatalf("required function has wrong type")
	}
	result, err := requiredFn("", nil)
	assert.Equal(t, "not implemented", result)
	require.NoError(t, err)

	lookupFn, ok := fm["lookup"].(func(string, string, string, string) (map[string]any, error))
	if !ok {
		t.Fatalf("lookup function has wrong type")
	}
	m, err := lookupFn("", "", "", "")
	assert.Equal(t, map[string]any{}, m)
	require.NoError(t, err)

	b64decFn, ok := fm["b64dec"].(func(string) (string, error))
	if !ok {
		t.Fatalf("b64dec function has wrong type")
	}
	b64result, b64err := b64decFn("")
	assert.Equal(t, "b64decDecoded_String", b64result)
	require.NoError(t, b64err)

	b32decFn, ok := fm["b32dec"].(func(string) (string, error))
	if !ok {
		t.Fatalf("b32dec function has wrong type")
	}
	b32result, b32err := b32decFn("")
	assert.Equal(t, "b32decDecoded_String", b32result)
	require.NoError(t, b32err)
}
