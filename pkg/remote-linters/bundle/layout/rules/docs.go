package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require docs/ so package documentation rules have a directory to inspect.

// DocsRuleID is the stable identifier used to reference this rule in configuration.
const DocsRuleID = "docs"

// NewDocsRule constructs a rule that requires docs/ in the package root.
func NewDocsRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(DocsRuleID), nil, []string{"docs"})
}
