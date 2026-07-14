package rules

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg/errors"
)

// Rule purpose: reject committed .helmignore because build generates it at runtime.

// NoHelmignoreRuleID is the stable identifier used to reference this rule in configuration.
const NoHelmignoreRuleID = "no-helmignore"

// helmignoreFile is the generated Helm ignore file that should not be committed.
const helmignoreFile = ".helmignore"

// NoHelmignoreRule reports .helmignore in the package root.
type NoHelmignoreRule struct {
	errorList *errors.LintRuleErrorsList
	path      string
}

// NewNoHelmignoreRule constructs a NoHelmignoreRule scoped to path, tagging diagnostics with the rule ID.
func NewNoHelmignoreRule(path string, errorList *errors.LintRuleErrorsList) *NoHelmignoreRule {
	return &NoHelmignoreRule{
		path:      path,
		errorList: errorList.WithRule(NoHelmignoreRuleID),
	}
}

// Check runs the .helmignore absence check against the package directory.
func (r *NoHelmignoreRule) Check(_ context.Context) {
	r.checkHelmignoreFile()
}

// checkHelmignoreFile reports an error if .helmignore is present.
func (r *NoHelmignoreRule) checkHelmignoreFile() {
	path := filepath.Join(r.path, helmignoreFile)

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return
	}

	if err != nil {
		return
	}

	r.errorList.WithFilePath(path).Errorf(".helmignore found - file is generated at build time")
}
