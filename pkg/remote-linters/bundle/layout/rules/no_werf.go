package rules

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg/errors"
)

// Rule purpose: reject package-level Werf files because custom builds must live in hooks/ or images/.

// werf artifact paths that must not exist in an application package.
const (
	werfDir  = ".werf"
	werfFile = "werf.yaml"
)

// NoWerfRuleID is the stable identifier used to reference this rule in configuration.
const NoWerfRuleID = "no-werf"

// NoWerfRule enforces that werf build artifacts are absent from the package root.
type NoWerfRule struct {
	errorList *errors.LintRuleErrorsList
	path      string
}

// NewNoWerfRule constructs a NoWerfRule scoped to path, tagging diagnostics with the rule ID.
func NewNoWerfRule(path string, errorList *errors.LintRuleErrorsList) *NoWerfRule {
	return &NoWerfRule{
		path:      path,
		errorList: errorList.WithRule(NoWerfRuleID),
	}
}

// Check runs both werf artifact checks against the package directory.
func (r *NoWerfRule) Check(_ context.Context) {
	r.checkWerfDir()
	r.checkWerfFile()
}

// checkWerfDir reports an error if the .werf directory is present.
func (r *NoWerfRule) checkWerfDir() {
	path := filepath.Join(r.path, werfDir)

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return
	}

	if err != nil {
		return
	}

	r.errorList.WithFilePath(path).Errorf(".werf directory found - custom build files allowed only in hooks/ or images/")
}

// checkWerfFile reports an error if werf.yaml is present.
func (r *NoWerfRule) checkWerfFile() {
	path := filepath.Join(r.path, werfFile)

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return
	}

	if err != nil {
		return
	}

	r.errorList.WithFilePath(path).Errorf("werf.yaml found - custom werf.yaml not allowed")
}
