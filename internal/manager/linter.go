package manager

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/errors"
)

type Linter interface {
	Run(m *module.Module) *errors.LintRuleErrorsList
	Name() string
}

type LinterList []Linter
