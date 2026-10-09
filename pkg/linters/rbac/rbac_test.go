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

package rbac

import (
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules"
)

// A rule at the ignored level is not run: --fix would otherwise rewrite files on behalf of findings
// nobody sees.
func TestRules_IgnoredRuleIsNotRun(t *testing.T) {
	for _, ignored := range []string{rules.ContractRuleName, rules.CoverageRuleName, rules.SyncRuleName} {
		t.Run(ignored, func(t *testing.T) {
			m := mocks.NewModuleMock(minimock.NewController(t))
			m.GetNameMock.Return("m")

			cfg := &pkg.RBACLinterConfig{}
			for name, rule := range map[string]*pkg.RuleConfig{
				rules.ContractRuleName: &cfg.Rules.ContractRule,
				rules.CoverageRuleName: &cfg.Rules.CoverageRule,
				rules.SyncRuleName:     &cfg.Rules.SyncRule,
			} {
				level := "warn"
				if name == ignored {
					level = "ignored"
				}

				rule.SetLevel(level, "warn")
			}

			names := map[string]bool{}
			for _, rule := range New(cfg, nil, m, errors.NewLintRuleErrorsList()).rules() {
				names[rule.GetName()] = true
			}

			for _, name := range []string{rules.ContractRuleName, rules.CoverageRuleName, rules.SyncRuleName} {
				assert.Equal(t, name != ignored, names[name], name)
			}
		})
	}
}
