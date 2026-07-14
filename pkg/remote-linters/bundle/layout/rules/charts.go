package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require charts directory so bundle image have a helm-lib with a helpers for templates.

// ChartsRuleID is the stable identifier used to reference this rule in configuration.
const ChartsRuleID = "charts"

// NewChartsRule constructs a rule that requires charts directory in the bundle root.
func NewChartsRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(ChartsRuleID), nil, []string{"charts"})
}
