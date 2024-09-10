package manager

import (
	"context"

	"github.com/deckhouse/d8-lint/pkg/errors"
)

type Linter interface {
	Run(ctx context.Context, m Module) (errors.LintRuleErrorsList, error)
	Name() string
	Desc() string
}
