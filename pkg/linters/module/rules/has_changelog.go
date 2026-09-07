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

// HasChangelogRule reports a module source tree that carries no changelog, or carries an
// empty one. Whether what it carries parses is changelog-valid's business, against the
// release image the file actually ships in.
type HasChangelogRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*HasChangelogRule)(nil)

func (r *HasChangelogRule) Check(_ context.Context) {
	path := filepath.Join(r.module.GetPath(), ChangelogFilename)
	errorList := r.errorList.WithFilePath(path)

	info, err := os.Stat(path)

	switch {
	case os.IsNotExist(err):
		errorList.Error("changelog.yaml file is missing")
	case err != nil:
		errorList.WithValue(err.Error()).Error("failed to check changelog.yaml file")
	case info.Size() == 0:
		errorList.Error("changelog.yaml file is empty")
	}
}
