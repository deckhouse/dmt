package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require module.yaml so package changes have a module definition.

// ModuleYAMLRuleID is the stable identifier used to reference this rule in configuration.
const ModuleYAMLRuleID = "module-definition"

// NewModuleYAMLRule constructs a rule that requires module.yaml in the package root.
func NewModuleYAMLRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(ModuleYAMLRuleID), []string{"module.yaml"}, nil)
}
