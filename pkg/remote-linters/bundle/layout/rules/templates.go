package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require templates/ so bundle image have a helm templates.

// TemplatesRuleID is the stable identifier used to reference this rule in configuration.
const TemplatesRuleID = "templates"

// NewTemplatesRule constructs a rule that requires templates/ in the bundle root.
func NewTemplatesRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(TemplatesRuleID), nil, []string{"templates"})
}
