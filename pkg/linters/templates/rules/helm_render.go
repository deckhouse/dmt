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
	"context"

	"github.com/deckhouse/dmt/internal/modules/render"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const HelmRenderRuleName = "helm-render"

func NewHelmRenderRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *HelmRenderRule {
	return &HelmRenderRule{
		RuleMeta:  pkg.RuleMeta{Name: HelmRenderRuleName},
		module:    m,
		errorList: errorList.WithRule(HelmRenderRuleName),
	}
}

type HelmRenderRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*HelmRenderRule)(nil)

// Check strictly renders module templates through nelm's public action.ChartRender
// and reports any rendering error as a lint finding. Strict rendering (no LintMode)
// catches template errors the main lenient render suppresses. Image references
// resolve from the module's pre-computed .Values (global.modulesImages, scanned
// from images/), so no helm_lib template override is needed here.
func (r *HelmRenderRule) Check(ctx context.Context) {
	m, errorList := r.module, r.errorList

	_, err := render.Render(ctx, m.GetNamespace(), m.GetName(), render.Options{
		Path:             m.GetPath(),
		Values:           m.GetValues(),
		ExtraAPIVersions: render.ExtraAPIVersions(),
	})
	if err != nil {
		errorList.Errorf("helm render failed: %s", err.Error())
	}
}
