package rules

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ChangelogRuleName = "changelog"
)

func NewChangelogRule() *ChangelogRule {
	return &ChangelogRule{
		RuleMeta: pkg.RuleMeta{
			Name: ChangelogRuleName,
		},
	}
}

type ChangelogRule struct {
	pkg.RuleMeta
	pkg.PathRule
}

func (r *ChangelogRule) CheckChangelog(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	path := filepath.Join(modulePath, "changelog.yaml")

	if _, err := os.Stat(path); err != nil {
		errorList.
			WithFilePath(path).
			Error("changelog.yaml file is missing")

		return
	}

	info, err := os.Stat(path)
	if err != nil {
		errorList.
			WithFilePath(path).
			WithValue(err.Error()).
			Error("failed to check changelog.yaml file")

		return
	}

	if info.Size() == 0 {
		errorList.
			WithFilePath(path).
			Error("changelog.yaml file is empty")
	}
}
