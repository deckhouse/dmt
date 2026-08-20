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

	"github.com/deckhouse/dmt/internal/fsutils"
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
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.OpenAPILinterConfig, m pkg.Module, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      ID,
		desc:      "Linter will check openapi values is correct",
		cfg:       cfg,
		module:    m,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (o *OpenAPI) Lint(ctx context.Context) {
	if o.module == nil {
		return
	}

	for _, rule := range o.rules() {
		rule.Check(ctx)
	}
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

	// Only BilingualRule has a real per-rule level: mapOpenAPIRules reads its
	// global impact. The other four are filled by mapSimpleLinterRules from the
	// linter impact New already applied, so scoping by them would be a no-op.
	return []pkg.Rule{
		rules.NewEnumRule(cfg, m, errorList),
		rules.NewHARule(cfg, m, errorList),
		rules.NewKeysRule(cfg, m, errorList),
		rules.NewDeckhouseCRDsRule(cfg, m, errorList),
		rules.NewBilingualRule(cfg, m, errorList.WithMaxLevel(cfg.Rules.BilingualRule.GetLevel())),
	}

	// doc-ru- YAML syntax check: doc-ru- files are excluded from the enum/ha/keys
	// parsing above, so this is the only place their YAML syntax is validated.
	docRuErrorList := errorLists.WithMaxLevel(o.cfg.Rules.DocRuYAMLRule.GetLevel())
	docRuValidator := rules.NewDocRuYAMLRule(o.cfg, m.GetPath())

	for _, file := range fsutils.GetFiles(m.GetPath(), true, filterDocRuYAMLfiles) {
		docRuValidator.Run(file, docRuErrorList)
	}
}

func (o *OpenAPI) Name() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}
