// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0

package docs

import (
	"context"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/docs/rules"
)

const (
	ID = "documentation"
)

// Documentation linter
type Documentation struct {
	name, desc string
	cfg        *pkg.DocumentationLinterConfig
	module     pkg.Module
	ruleIDs    set.Set
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.DocumentationLinterConfig, ruleIDs set.Set, m pkg.Module, errorList *errors.LintRuleErrorsList) *Documentation {
	return &Documentation{
		name:      ID,
		desc:      "Documentation linter checks module documentation requirements",
		cfg:       cfg,
		module:    m,
		ruleIDs:   ruleIDs,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (l *Documentation) Lint(ctx context.Context) {
	if l.module.GetPath() == "" {
		return
	}

	pkg.RunRules(ctx, l.ruleIDs, l.rules())
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
func (l *Documentation) rules() []pkg.Rule {
	m := l.module
	errorList := l.ErrorList.WithModule(m.GetName())

	return []pkg.Rule{
		rules.NewReadmeRule(m, errorList.WithMaxLevel(l.cfg.Rules.ReadmeRule.GetLevel())),
		rules.NewBilingualRule(m, errorList.WithMaxLevel(l.cfg.Rules.BilingualRule.GetLevel())),
		rules.NewCyrillicInEnglishRule(m, errorList.WithMaxLevel(l.cfg.Rules.CyrillicInEnglishRule.GetLevel())),
		rules.NewNoLangKeyRule(m, errorList.WithMaxLevel(l.cfg.Rules.NoLangKeyRule.GetLevel())),
		rules.NewMarkdownRule(m, errorList.WithMaxLevel(l.cfg.Rules.MarkdownlintRule.GetLevel())),
		rules.NewSizeRule(m, errorList.WithMaxLevel(l.cfg.Rules.SizeRule.GetLevel())),
		rules.NewFrontMatterRule(m, errorList.WithMaxLevel(l.cfg.Rules.FrontMatterRule.GetLevel())),
		rules.NewChangelogRule(m, errorList.WithMaxLevel(l.cfg.Rules.ChangelogRule.GetLevel())),
	}
}

func (l *Documentation) GetName() string {
	return l.name
}

func (l *Documentation) Desc() string {
	return l.desc
}
