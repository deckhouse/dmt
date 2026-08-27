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
	"context"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/openapi/rules"
)

const (
	ID = "openapi"
)

// OpenAPI linter
type OpenAPI struct {
	name, desc string
	cfg        *pkg.OpenAPILinterConfig
	module     pkg.Module
	ruleIDs    set.Set
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.OpenAPILinterConfig, ruleIDs set.Set, m pkg.Module, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      ID,
		desc:      "Linter will check openapi values is correct",
		cfg:       cfg,
		module:    m,
		ruleIDs:   ruleIDs,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (o *OpenAPI) Lint(ctx context.Context) {
	pkg.RunRules(ctx, o.ruleIDs, o.rules())
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
//
// Each rule now owns its own file discovery: enum and high-availability walk
// openapi/ plus crds/, keys and deckhouse-crds only crds/, and bilingual only
// the top level of crds/.
func (o *OpenAPI) rules() []pkg.Rule {
	m := o.module
	cfg := o.cfg
	errorList := o.ErrorList.WithModule(m.GetName())

	// The enum/ha/keys/deckhouse-crds rules have no real per-rule level:
	// mapSimpleLinterRules fills them from the linter impact New already applied,
	// so scoping by them would be a no-op. The last three do carry one.
	return []pkg.Rule{
		rules.NewEnumRule(cfg, m, errorList),
		rules.NewHARule(cfg, m, errorList),
		rules.NewKeysRule(cfg, m, errorList),
		rules.NewDeckhouseCRDsRule(cfg, m, errorList),
		rules.NewBilingualRule(cfg, m, errorList.WithMaxLevel(cfg.Rules.BilingualRule.GetLevel())),
		// x-deckhouse-validations / x-kubernetes-validations CEL blocks. Scoped to
		// openapi/ only, inside the rule: in crds/, x-kubernetes-validations is a
		// legitimate, API-server-honored construct and must not be flagged.
		rules.NewDeckhouseValidationsRule(cfg, m, errorList.WithMaxLevel(cfg.Rules.DeckhouseValidationsRule.GetLevel())),
		// doc-ru- files are excluded from the enum/ha/keys parsing above, so this is
		// the only place their YAML syntax is validated.
		rules.NewDocRuYAMLRule(cfg, m, errorList.WithMaxLevel(cfg.Rules.DocRuYAMLRule.GetLevel())),
	}
}

// AllRuleNames returns the IDs of every rule this linter has. It is not knowledge about
// scopes: the linter only states honestly what it carries. Checking the list against a
// scope's table is done in pkg/scopes, not here.
func AllRuleNames() set.Set {
	return set.New(
		rules.BilingualRuleName,
		rules.CRDsRuleName,
		rules.DeckhouseValidationsRuleName,
		rules.DocRuYAMLRuleName,
		rules.EnumRuleName,
		rules.HARuleName,
		rules.KeysRuleName,
	)
}

func (o *OpenAPI) GetName() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}
