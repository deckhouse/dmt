package rules

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg/errors"
)

// Rule purpose: require a non-empty changelog.yaml as the release changelog entry point.

// ChangelogRuleID is the stable identifier used to reference this rule in configuration.
const ChangelogRuleID = "changelog"

// ChangelogRule enforces that changelog.yaml exists and is not empty.
type ChangelogRule struct {
	path      string
	errorList *errors.LintRuleErrorsList
}

// NewChangelogRule constructs a ChangelogRule scoped to path, tagging diagnostics with the rule ID.
func NewChangelogRule(path string, errorList *errors.LintRuleErrorsList) *ChangelogRule {
	return &ChangelogRule{
		path:      path,
		errorList: errorList.WithRule(ChangelogRuleID),
	}
}

// Check verifies that changelog.yaml exists and has content.
func (r *ChangelogRule) Check(_ context.Context) {
	path := filepath.Join(r.path, "changelog.yaml")

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		r.errorList.
			WithFilePath(path).
			WithValue(err.Error()).
			Error("changelog.yaml file is missing")

		return
	}

	if err != nil {
		r.errorList.
			WithFilePath(path).
			WithValue(err.Error()).
			Error("failed to check changelog.yaml file")

		return
	}

	if info.Size() == 0 {
		r.errorList.
			WithFilePath(path).
			WithValue("file is empty").
			Error("changelog.yaml file is empty")
	}
}
