package rules

import "github.com/deckhouse/dmt/pkg/errors"

// Rule purpose: require .gitignore so local and generated files stay out of package repositories.

// GitignoreRuleID is the stable identifier used to reference this rule in configuration.
const GitignoreRuleID = "gitignore"

// NewGitignoreRule constructs a rule that requires .gitignore in the package root.
func NewGitignoreRule(path string, errorList *errors.LintRuleErrorsList) *requiredRootPathsRule {
	return newRequiredRootPathsRule(path, errorList.WithRule(GitignoreRuleID), []string{".gitignore"}, nil)
}
