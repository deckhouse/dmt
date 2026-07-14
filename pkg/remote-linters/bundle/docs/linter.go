package docs

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/remote-linters/bundle/docs/rules"
)

// LinterID is the stable identifier used to reference this linter in configuration and diagnostics.
const LinterID = "docs"

// Linter runs documentation rules against a package directory.
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
		errorList: errorList.WithLinterID(LinterID),
	}
}

// Lint executes all documentation rules against the configured package path.
func (l *Linter) Lint(ctx context.Context) {
	if !hasDocsDir(l.config.Path) {
		l.errorList.WithFilePath(l.config.Path).Warn("docs folder not found in package root")
		return
	}

	rules.NewReadmeRule(l.config.Path, l.errorList).Check(ctx)
	// rules.NewBilingualRule(l.config.Path).Check(ctx)
	// rules.NewCyrillicInEnglishRule(l.config.Path).Check(ctx)
}

// hasDocsDir reports whether docs/ exists as a directory in the package root.
func hasDocsDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, "docs"))
	return err == nil && info.IsDir()
}
