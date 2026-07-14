package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require Chart.yaml so bundle image have a chart definition.

// ChartYAMLRuleID is the stable identifier used to reference this rule in configuration.
const ChartYAMLRuleID = "chart-yaml"

// NewChartYAMLRule constructs a rule that requires Chart.yaml in the bundle root.
func NewChartYAMLRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(ChartYAMLRuleID), []string{"Chart.yaml"}, nil)
}
