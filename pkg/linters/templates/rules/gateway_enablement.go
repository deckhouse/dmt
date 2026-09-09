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
	GatewayEnablementRuleName = "gateway-enablement"

	// gatewayEnabledHelper is the shared helm_lib helper that decides whether a
	// module's Gateway API resources (HTTPRoute, ListenerSet) should be created:
	// it checks the module's own `<module>.gatewayAPI.enabled` override (or the
	// global `global.modules.gatewayAPI.enabled`), AND requires that a Gateway
	// actually resolves (module, then global, then
	// global.discovery.gatewayAPIDefaultGateway) — unlike Ingress there is no
	// safe default gateway, so both conditions matter.
	gatewayEnabledHelper = "helm_lib_module_gateway_enabled"
)

type GatewayEnablementRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewGatewayEnablementRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *GatewayEnablementRule {
	return &GatewayEnablementRule{
		RuleMeta: pkg.RuleMeta{
			Name: GatewayEnablementRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
		},
		module:    m,
		errorList: errorList.WithRule(GatewayEnablementRuleName),
	}
}

var _ pkg.Rule = (*GatewayEnablementRule)(nil)

// Check scans every template file that emits a `kind: HTTPRoute` or
// `kind: ListenerSet` manifest and reports the ones that never reference
// helm_lib_module_gateway_enabled anywhere in the same file. See
// IngressEnablementRule.Check for the same-file heuristic this shares and why it
// was chosen.
//
// The rule only runs when the module actually renders an HTTPRoute or
// ListenerSet: a module with neither has nothing for this check to say.
func (r *GatewayEnablementRule) Check(_ context.Context) {
	if !storageHasKind(r.module, "HTTPRoute", "ListenerSet") {
		return
	}

	camelModuleName := modules.ToLowerCamel(r.module.GetName())
	valuesHint := fmt.Sprintf(
		"global.modules.gatewayAPI.enabled or %[1]s.gatewayAPI.enabled, with a Gateway resolvable "+
			"via global.discovery.gatewayAPIDefaultGateway, global.modules.gatewayAPI.gateway, or %[1]s.gatewayAPI.gateway",
		camelModuleName,
	)

	checkKindGatedByHelper(
		r.module, r.errorList, r.PathRule,
		kindLineRe("HTTPRoute", "ListenerSet"), gatewayEnabledHelper,
		"Gateway API (HTTPRoute/ListenerSet)", valuesHint,
	)
}
