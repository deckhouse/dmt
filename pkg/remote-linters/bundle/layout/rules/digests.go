package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require images_digests.json so bundle image have a digests for all images.

// ChartsRuleID is the stable identifier used to reference this rule in configuration.
const DigestsRuleID = "digests"

// NewDigestsRule constructs a rule that requires images_digests.json in the bundle root.
func NewDigestsRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(DigestsRuleID), []string{"images_digests.json"}, nil)
}
