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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestRenderChartFromRawValues(t *testing.T) {
	t.Run("should return error if chart name is empty", func(t *testing.T) {
		renderer := Renderer{
			Name:      "",
			Namespace: "default",
			LintMode:  false,
		}

		_, err := renderer.RenderChartFromRawValues(&chart.Chart{}, chartutil.Values{})
		require.Error(t, err)
		assert.Equal(t, "helm chart must have a name", err.Error())
	})

	t.Run("should render chart successfully", func(t *testing.T) {
		renderer := Renderer{
			Name:      "test-chart",
			Namespace: "default",
			LintMode:  false,
		}

		testChart := &chart.Chart{
			Metadata: &chart.Metadata{
				Name: "test-chart",
			},
			Templates: []*chart.File{
				{
					Name: "templates/test.yaml",
					Data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test-configmap"),
				},
			},
		}

		values := chartutil.Values{
			"key": "value",
		}

		output, err := renderer.RenderChartFromRawValues(testChart, values)
		require.NoError(t, err)
		assert.Contains(t, output, "test-chart/templates/test.yaml")
		assert.Contains(t, output["test-chart/templates/test.yaml"], "apiVersion: v1")
		assert.Contains(t, output["test-chart/templates/test.yaml"], "kind: ConfigMap")
		assert.Contains(t, output["test-chart/templates/test.yaml"], "name: test-configmap")
	})
}
