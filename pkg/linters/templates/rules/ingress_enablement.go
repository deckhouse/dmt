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
	"fmt"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	IngressEnablementRuleName = "ingress-enablement"

	// ingressEnabledHelper is the shared helm_lib helper that decides whether a
	// module's Ingress should be created: it checks the module's own
	// `<module>.ingress.enabled` override first, then falls back to the global
	// `global.modules.ingress.enabled`, defaulting to true when neither is set.
	ingressEnabledHelper = "helm_lib_module_ingress_enabled"
)

type IngressEnablementRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewIngressEnablementRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *IngressEnablementRule {
	return &IngressEnablementRule{
		RuleMeta: pkg.RuleMeta{
			Name: IngressEnablementRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
		},
		module:    m,
		errorList: errorList.WithRule(IngressEnablementRuleName),
	}
}

var _ pkg.Rule = (*IngressEnablementRule)(nil)

// Check scans every template file that emits a `kind: Ingress` manifest and
// reports the ones that never reference helm_lib_module_ingress_enabled anywhere
// in the same file. Without that helper (or an equivalent check on the same
// values), the Ingress renders unconditionally and cannot be turned off via
// either global.modules.ingress.enabled or the module's own ingress.enabled
// override — the two supported ways to disable it.
//
// This is a textual, same-file heuristic, not a template-scope analysis: a file
// that emits several Ingress manifests but only guards one of them with the
// helper will not be flagged. In every module observed so far the guard and the
// manifest it protects live in the same file, so this trade-off catches the
// common and important case — an Ingress with no enablement check at all —
// without the cost of a real Helm-template control-flow parser.
//
// The rule only runs when the module actually renders an Ingress object: a
// module with none has nothing for this check to say.
func (r *IngressEnablementRule) Check(_ context.Context) {
	if !storageHasKind(r.module, "Ingress") {
		return
	}

	camelModuleName := modules.ToLowerCamel(r.module.GetName())
	valuesHint := fmt.Sprintf("global.modules.ingress.enabled or %s.ingress.enabled", camelModuleName)

	checkKindGatedByHelper(
		r.module, r.errorList, r.PathRule,
		kindLineRe("Ingress"), ingressEnabledHelper,
		"Ingress", valuesHint,
	)
}
