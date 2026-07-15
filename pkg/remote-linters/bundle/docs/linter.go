package docs

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/remote-linters/bundle/docs/rules"
)

// LinterID is the stable identifier used to reference this linter in configuration and diagnostics.
const LinterID = "docs"

// Linter runs documentation rules against a package directory.
type Linter struct {
	path      string
	cfg       *pkg.DocumentationLinterConfig
	errorList *errors.LintRuleErrorsList
}

// NewLinter constructs a Linter from cfg, scoping its diagnostics to this linter and capping severity at the configured level.
func NewLinter(path string, cfg *pkg.DocumentationLinterConfig, errorList *errors.LintRuleErrorsList) *Linter {
	return &Linter{
		path:      path,
		cfg:       cfg,
		errorList: errorList.WithLinterID(LinterID).WithMaxLevel(cfg.Impact),
	}
}

// Lint executes all documentation rules against the configured package path.
func (l *Linter) Lint(ctx context.Context) {
	if !hasDocsDir(l.path) {
		l.errorList.WithFilePath(l.path).Warn("docs folder not found in package root")
		return
	}

	rules.NewReadmeRule(l.path, l.errorList.WithMaxLevel(l.cfg.Rules.ReadmeRule.GetLevel())).Check(ctx)
	rules.NewBilingualRule(l.path, l.errorList.WithMaxLevel(l.cfg.Rules.BilingualRule.GetLevel())).Check(ctx)
}

// hasDocsDir reports whether docs/ exists as a directory in the package root.
func hasDocsDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, "docs"))
	return err == nil && info.IsDir()
}
