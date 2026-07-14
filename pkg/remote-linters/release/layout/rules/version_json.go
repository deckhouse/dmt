package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require version.json so package changes have a version definition.

// VersionJSONRuleID is the stable identifier used to reference this rule in configuration.
const VersionJSONRuleID = "version-json"

// NewVersionJSONRule constructs a rule that requires version.json in the package root.
func NewVersionJSONRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(VersionJSONRuleID), []string{"version.json"}, nil)
}
