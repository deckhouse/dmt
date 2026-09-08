// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	HasChangelogRuleName = "has-changelog"
)

func NewHasChangelogRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *HasChangelogRule {
	return &HasChangelogRule{
		RuleMeta: pkg.RuleMeta{
			Name: HasChangelogRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(HasChangelogRuleName),
	}
}

// HasChangelogRule reports a published image that carries no changelog, or carries an empty one.
type HasChangelogRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*HasChangelogRule)(nil)

func (r *HasChangelogRule) Check(_ context.Context) {
	errorList := r.errorList.WithFilePath(ChangelogFilename)

	info, err := os.Stat(filepath.Join(r.module.GetPath(), ChangelogFilename))

	switch {
	case os.IsNotExist(err):
		errorList.Warn("changelog.yaml file is missing")
	case err != nil:
		errorList.WithValue(err.Error()).Warn("failed to check changelog.yaml file")
	case info.Size() == 0:
		errorList.Warn("changelog.yaml file is empty")
	}
}
