package rules

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg/errors"
)

// Rule purpose: require a non-empty docs/README.md as the package documentation entry point.

// ReadmeRuleID is the stable identifier used to reference this rule in configuration.
const ReadmeRuleID = "readme"

// ReadmeRule enforces that docs/README.md exists and is not empty.
type ReadmeRule struct {
	path      string
	errorList *errors.LintRuleErrorsList
}

// NewReadmeRule constructs a ReadmeRule scoped to path, tagging diagnostics with the rule ID.
func NewReadmeRule(path string, errorList *errors.LintRuleErrorsList) *ReadmeRule {
	return &ReadmeRule{
		path:      path,
		errorList: errorList.WithRule(ReadmeRuleID),
	}
}

// Check verifies that docs/README.md exists and has content.
func (r *ReadmeRule) Check(_ context.Context) {
	path := filepath.Join(r.path, "docs", "README.md")

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		r.errorList.
			WithFilePath(path).
			WithValue(err.Error()).
			Error("README.md file is missing in docs/ directory")
		return
	}

	if err != nil {
		r.errorList.
			WithFilePath(path).
			WithValue(err.Error()).
			Error("failed to check README.md file")

		return
	}

	if info.Size() == 0 {
		r.errorList.
			WithFilePath(path).
			WithValue("file is empty").
			Error("README.md file is empty")
	}
}
