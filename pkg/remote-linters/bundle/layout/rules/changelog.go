package rules

import (
	"github.com/deckhouse/dmt/pkg/errors"
)

// Rule purpose: require changelog.yaml so bundle image have a release history entry point.

// ChangelogRuleID is the stable identifier used to reference this rule in configuration.
const ChangelogRuleID = "changelog"

// NewChangelogRule constructs a rule that requires changelog.yaml in the package root.
func NewChangelogRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(ChangelogRuleID), []string{"changelog.yaml"}, nil)
}
