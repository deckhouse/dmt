/*
Copyright 2026 Flant JSC

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

package rules

import (
	"github.com/deckhouse/dmt/internal/helm"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const HelmRenderRuleName = "helm-render"

func NewHelmRenderRule() *HelmRenderRule {
	return &HelmRenderRule{
		RuleMeta: pkg.RuleMeta{Name: HelmRenderRuleName},
	}
}

type HelmRenderRule struct {
	pkg.RuleMeta
}

// Check renders module templates with nelm and reports any rendering errors.
// It uses the module's pre-computed render values (from openapi schemas) and
// reports rendering failures as lint findings via the errorList.
func (r *HelmRenderRule) Check(m *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	renderer := helm.Renderer{
		Name:             m.GetName(),
		Namespace:        m.GetNamespace(),
		LintMode:         true,
		HelmLibOverrides: module.HelmLibOverrides(),
	}

	_, err := renderer.RenderChartFromDir(m.GetPath(), m.GetValues())
	if err != nil {
		errorList.Errorf("helm render failed: %s", err.Error())
	}
}
