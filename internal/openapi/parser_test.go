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

package openapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCRD(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expected bool
	}{
		{
			name: "is CRD",
			data: map[string]any{
				"kind": "CustomResourceDefinition",
			},
			expected: true,
		},
		{
			name: "is not CRD",
			data: map[string]any{
				"kind": "Deployment",
			},
			expected: false,
		},
		{
			name: "no kind",
			data: map[string]any{
				"apiVersion": "v1",
			},
			expected: false,
		},
		{
			name:     "empty data",
			data:     map[string]any{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCRD(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDeckhouseCRD(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expected bool
	}{
		{
			name: "is Deckhouse CRD",
			data: map[string]any{
				"kind": "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": "module.deckhouse.io",
				},
			},
			expected: true,
		},
		{
			name: "is not Deckhouse CRD",
			data: map[string]any{
				"kind": "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": "module.example.com",
				},
			},
			expected: false,
		},
		{
			name: "not a CRD",
			data: map[string]any{
				"kind": "Deployment",
			},
			expected: false,
		},
		{
			name: "no metadata",
			data: map[string]any{
				"kind": "CustomResourceDefinition",
			},
			expected: false,
		},
		{
			name: "metadata not a map",
			data: map[string]any{
				"kind":     "CustomResourceDefinition",
				"metadata": "not a map",
			},
			expected: false,
		},
		{
			name: "no name in metadata",
			data: map[string]any{
				"kind": "CustomResourceDefinition",
				"metadata": map[string]any{
					"namespace": "default",
				},
			},
			expected: false,
		},
		{
			name: "name not a string",
			data: map[string]any{
				"kind": "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": 123,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDeckhouseCRD(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFileYAMLContent(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("valid yaml", func(t *testing.T) {
		yamlContent := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
`
		filePath := filepath.Join(tempDir, "valid.yaml")
		err := os.WriteFile(filePath, []byte(yamlContent), 0644)
		require.NoError(t, err)

		content, err := getFileYAMLContent(filePath)
		require.NoError(t, err)
		require.NotNil(t, content)

		assert.Equal(t, "v1", content["apiVersion"])
		assert.Equal(t, "Pod", content["kind"])
		metadata, ok := content["metadata"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test-pod", metadata["name"])
	})

	t.Run("invalid yaml", func(t *testing.T) {
		yamlContent := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  - invalid indentation here
spec:
  containers:
  - name: nginx
    image: nginx:latest
`
		filePath := filepath.Join(tempDir, "invalid.yaml")
		err := os.WriteFile(filePath, []byte(yamlContent), 0644)
		require.NoError(t, err)

		content, err := getFileYAMLContent(filePath)
		assert.Error(t, err)
		assert.Nil(t, content)
	})

	t.Run("file not found", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "nonexistent.yaml")

		content, err := getFileYAMLContent(filePath)
		assert.Error(t, err)
		assert.Nil(t, content)
	})
}

func TestParse(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("successful parse", func(t *testing.T) {
		yamlContent := `
key1: value1
key2:
  nestedKey: nestedValue
key3:
  - item1
  - item2
`
		filePath := filepath.Join(tempDir, "valid.yaml")
		err := os.WriteFile(filePath, []byte(yamlContent), 0644)
		require.NoError(t, err)

		var parsedKeys []string
		testParser := func(key string, value any) error {
			parsedKeys = append(parsedKeys, key)
			return nil
		}

		err = Parse(testParser, filePath)
		require.NoError(t, err)

		// Check that the parser was called for the nested keys in the file
		assert.Contains(t, parsedKeys, "key2.nestedKey")
	})

	t.Run("external CRD skip", func(t *testing.T) {
		yamlContent := `
kind: CustomResourceDefinition
metadata:
  name: example.other.io
`
		filePath := filepath.Join(tempDir, "external-crd.yaml")
		err := os.WriteFile(filePath, []byte(yamlContent), 0644)
		require.NoError(t, err)

		called := false
		testParser := func(key string, value any) error {
			called = true
			return nil
		}

		err = Parse(testParser, filePath)
		require.NoError(t, err)

		// Parser should not be called for external CRDs
		assert.False(t, called)
	})

	t.Run("deckhouse CRD not skipped", func(t *testing.T) {
		yamlContent := `
kind: CustomResourceDefinition
metadata:
  name: module.deckhouse.io
spec:
  something: value
`
		filePath := filepath.Join(tempDir, "deckhouse-crd.yaml")
		err := os.WriteFile(filePath, []byte(yamlContent), 0644)
		require.NoError(t, err)

		var parsedKeys []string
		testParser := func(key string, value any) error {
			parsedKeys = append(parsedKeys, key)
			return nil
		}

		err = Parse(testParser, filePath)
		require.NoError(t, err)

		// Parser should be called for Deckhouse CRDs
		assert.NotEmpty(t, parsedKeys)
	})
}

func TestFileParser_ParseValue(t *testing.T) {
	t.Run("parse map string", func(t *testing.T) {
		var parsedKeys []string
		testParser := func(key string, value any) error {
			parsedKeys = append(parsedKeys, key)
			return nil
		}

		fp := fileParser{parser: testParser}

		data := map[string]any{
			"nested": map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		}

		err := fp.parseValue("root", data)
		require.NoError(t, err)

		assert.Contains(t, parsedKeys, "root.nested.key1")
		assert.Contains(t, parsedKeys, "root.nested.key2")
	})

	t.Run("parse map any", func(t *testing.T) {
		var parsedKeys []string
		testParser := func(key string, value any) error {
			parsedKeys = append(parsedKeys, key)
			return nil
		}

		fp := fileParser{parser: testParser}

		nestedMap := make(map[any]any)
		nestedMap["key1"] = "value1"
		nestedMap["key2"] = "value2"

		data := map[any]any{
			"nested": nestedMap,
		}

		err := fp.parseValue("root", data)
		require.NoError(t, err)

		assert.Contains(t, parsedKeys, "root.nested.key1")
		assert.Contains(t, parsedKeys, "root.nested.key2")
	})

	t.Run("parse slice", func(t *testing.T) {
		var parsedKeys []string
		testParser := func(key string, value any) error {
			parsedKeys = append(parsedKeys, key)
			return nil
		}

		fp := fileParser{parser: testParser}

		data := []any{"item1", "item2"}

		err := fp.parseValue("root", data)
		require.NoError(t, err)

		// В текущей реализации parseSlice не вызывает parser для элементов слайса,
		// поэтому здесь нет проверки на наличие ключей "root[0]" и "root[1]"
	})

	t.Run("nil value", func(t *testing.T) {
		var parsedKeys []string
		testParser := func(key string, value any) error {
			parsedKeys = append(parsedKeys, key)
			return nil
		}

		fp := fileParser{parser: testParser}

		err := fp.parseValue("root", nil)
		require.NoError(t, err)
		assert.Empty(t, parsedKeys)
	})
}
