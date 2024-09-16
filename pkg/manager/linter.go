package manager

import (
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/module"
)

type Linter interface {
	Run(m *module.Module) (errors.LintRuleErrorsList, error)
	Name() string
	Desc() string
}

type LinterList []Linter
