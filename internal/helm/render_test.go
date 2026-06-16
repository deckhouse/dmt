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

package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeChart(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	return dir
}

func TestRenderChartFromDir(t *testing.T) {
	t.Run("should return error if chart name is empty", func(t *testing.T) {
		renderer := Renderer{
			Name:      "",
			Namespace: "default",
		}

		_, err := renderer.RenderChartFromDir(t.TempDir(), map[string]any{})
		require.Error(t, err)
		assert.Equal(t, "helm chart must have a name", err.Error())
	})

	t.Run("should render chart successfully", func(t *testing.T) {
		renderer := Renderer{
			Name:      "test-chart",
			Namespace: "default",
		}

		dir := writeChart(t, map[string]string{
			"Chart.yaml": "apiVersion: v2\nname: test-chart\nversion: 0.1.0\n",
			"templates/test.yaml": "apiVersion: v1\n" +
				"kind: ConfigMap\n" +
				"metadata:\n" +
				"  name: test-configmap\n" +
				"data:\n" +
				"  key: {{ .Values.key }}\n",
		})

		output, err := renderer.RenderChartFromDir(dir, map[string]any{
			"Values": map[string]any{"key": "value"},
		})
		require.NoError(t, err)
		require.Contains(t, output, "test-chart/templates/test.yaml")

		manifest := output["test-chart/templates/test.yaml"]
		assert.Contains(t, manifest, "kind: ConfigMap")
		assert.Contains(t, manifest, "name: test-configmap")
		assert.Contains(t, manifest, "key: value")
	})
}
