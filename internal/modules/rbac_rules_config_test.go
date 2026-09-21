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

package modules

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
)

// The rules added for the module RBAC declaration read their own impact from the root
// configuration; the four original rbac rules keep the linter level whatever the root says.
func TestRemapLinterSettings_RBACDeclarationRules(t *testing.T) {
	t.Run("per-rule levels from the root configuration, warn as the fallback", func(t *testing.T) {
		settings := remapLinterSettings(
			&config.LintersSettings{Rbac: config.RbacSettings{Impact: pkg.Error.String()}},
			&global.Linters{Rbac: global.RbacLinterConfig{
				LinterConfig: global.LinterConfig{Impact: pkg.Error.String()},
				Rules: global.RbacRules{
					CoverageRule: global.RuleConfig{Impact: pkg.Warn.String()},
					SyncRule:     global.RuleConfig{Impact: pkg.Ignored.String()},
				},
			}},
		)

		require.Equal(t, pkg.Warn, *settings.RBAC.Rules.CoverageRule.GetLevel())
		require.Equal(t, pkg.Ignored, *settings.RBAC.Rules.SyncRule.GetLevel())
		require.Equal(t, pkg.Warn, *settings.RBAC.Rules.ContractRule.GetLevel(), "unset starts at warn, whatever the linter level says")

		// SC5: the original rules are untouched by the per-rule block.
		for _, rule := range []*pkg.RuleConfig{
			&settings.RBAC.Rules.UserAuthRule, &settings.RBAC.Rules.BindingRule, &settings.RBAC.Rules.PlacementRule, &settings.RBAC.Rules.WildcardsRule,
		} {
			require.Equal(t, pkg.Error, *rule.GetLevel())
		}
	})

	t.Run("module-level exclusions for the three rules", func(t *testing.T) {
		settings := remapLinterSettings(
			&config.LintersSettings{Rbac: config.RbacSettings{ExcludeRules: config.RBACExcludeRules{
				Coverage: config.StringRuleExcludeList{"deckhouse.io/internals"},
				Contract: config.KindRuleExcludeList{{Kind: "ClusterRole", Name: "d8:namespace-capability:x:view"}},
				Sync:     config.KindRuleExcludeList{{Kind: "ClusterRole", Name: "d8:user-authz:x:user"}},
			}}},
			&global.Linters{},
		)

		require.Equal(t, pkg.StringRuleExcludeList{"deckhouse.io/internals"}, settings.RBAC.ExcludeRules.Coverage)
		require.Len(t, settings.RBAC.ExcludeRules.Contract.Get(), 1)
		require.Len(t, settings.RBAC.ExcludeRules.Sync.Get(), 1)
	})
}
