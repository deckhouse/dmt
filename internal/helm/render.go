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
	"fmt"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/engine"
)

type Renderer struct {
	Name      string
	Namespace string
	LintMode  bool
}

func (r Renderer) RenderChartFromRawValues(c *chart.Chart, values chartutil.Values) (map[string]string, error) {
	if r.Name == "" {
		return nil, fmt.Errorf("helm chart must have a name")
	}
	// render chart with prepared values
	var e engine.Engine
	e.LintMode = r.LintMode

	out, err := e.Render(c, values)
	if err != nil {
		return nil, err
	}

	return out, nil
}
