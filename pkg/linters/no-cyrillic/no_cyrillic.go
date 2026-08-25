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

package nocyrillic

import (
	"context"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/no-cyrillic/rules"
)

const (
	ID = "no-cyrillic"
)

// NoCyrillic linter
type NoCyrillic struct {
	name, desc string
	cfg        *pkg.NoCyrillicLinterConfig
	module     pkg.Module
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.NoCyrillicLinterConfig, m pkg.Module, errorList *errors.LintRuleErrorsList) *NoCyrillic {
	return &NoCyrillic{
		name:      ID,
		desc:      "NoCyrillic will check all files in the modules for contains cyrillic symbols",
		cfg:       cfg,
		module:    m,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (l *NoCyrillic) Lint(ctx context.Context) {
	for _, rule := range l.rules() {
		rule.Check(ctx)
	}
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
func (l *NoCyrillic) rules() []pkg.Rule {
	m := l.module

	// The rule gets the linter-level error list unchanged: cfg.Rules.NoCyrillicRule
	// is fed from the linter impact (mapSimpleLinterRules in internal/modules),
	// which New already applied, so scoping by it here would be a no-op.
	return []pkg.Rule{
		rules.NewFilesRule(
			l.cfg.ExcludeRules.Files.Get(),
			l.cfg.ExcludeRules.Directories.Get(),
			m, l.ErrorList.WithModule(m.GetName())),
	}
}

func (l *NoCyrillic) GetName() string {
	return l.name
}

func (l *NoCyrillic) Desc() string {
	return l.desc
}
