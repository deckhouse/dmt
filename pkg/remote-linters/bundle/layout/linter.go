package layout

import (
	"context"

	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/remote-linters/bundle/layout/rules"
)

// LinterID is the stable identifier used to reference this linter in configuration and diagnostics.
const LinterID = "layout"

// Linter runs layout rules against an application package directory.
type Linter struct {
	config    Config
	errorList *errors.LintRuleErrorsList
}

// Config holds the path and settings required to construct a Linter.
type Config struct {
	Path string
}

// NewLinter constructs a Linter from cfg, scoping its diagnostics to this linter and capping severity at the configured level.
func NewLinter(cfg Config, errorList *errors.LintRuleErrorsList) *Linter {
	return &Linter{
		config:    cfg,
		errorList: errorList.WithRule(LinterID),
	}
}

// Lint executes all layout rules against the configured package path.
func (l *Linter) Lint(ctx context.Context) {
	rules.NewNoWerfRule(l.config.Path, l.errorList).Check(ctx)
	rules.NewNoChartRule(l.config.Path, l.errorList).Check(ctx)
	rules.NewNoHelmignoreRule(l.config.Path, l.errorList).Check(ctx)
	rules.NewGitignoreRule(l.config.Path, l.errorList).Check(ctx)
	rules.NewChangelogRule(l.config.Path, l.errorList).Check(ctx)
	rules.NewDocsRule(l.config.Path, l.errorList).Check(ctx)
}
