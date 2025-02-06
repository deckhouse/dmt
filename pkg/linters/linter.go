package linters

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

type Linter interface {
	Run(m *module.Module) *errors.LintRuleErrorsList
	Name() string
}

type LinterList []func(cfg *config.ModuleConfig, errList *errors.LintRuleErrorsList) Linter
