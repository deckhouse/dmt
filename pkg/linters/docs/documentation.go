// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0

package docs

import (
	"github.com/deckhouse/dmt/internal/module"
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
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.DocumentationLinterConfig, errorList *errors.LintRuleErrorsList) *Documentation {
	return &Documentation{
		name:      ID,
		desc:      "Documentation linter checks module documentation requirements",
		cfg:       cfg,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (l *Documentation) Run(m *module.Module) {
	if m == nil || m.GetPath() == "" {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	rules.NewReadmeRule(
		l.cfg.ExcludeRules.Readme.Modules.Get(),
	).CheckReadme(m, errorList)

	rules.NewBilingualRule(
		l.cfg.ExcludeRules.Bilingual.Modules.Get(),
		l.cfg.BilingualRule.Disable,
	).CheckBilingual(m, errorList)

	rules.NewCyrillicInEnglishRule(
		l.cfg.ExcludeRules.CyrillicInEnglish.Files.Get(),
		l.cfg.ExcludeRules.CyrillicInEnglish.Directories.Get(),
	).CheckFiles(m, errorList)
}

func (l *Documentation) Name() string {
	return l.name
}

func (l *Documentation) Desc() string {
	return l.desc
}
