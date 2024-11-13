package manager

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

type Linter interface {
	Run(m *module.Module) (errors.LintRuleErrorsList, error)
	Name() string
	Desc() string
}

type LinterList []Linter
