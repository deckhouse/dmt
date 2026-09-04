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
	ChangelogRuleName = "changelog"
)

func NewChangelogRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *ChangelogRule {
	return &ChangelogRule{
		RuleMeta: pkg.RuleMeta{
			Name: ChangelogRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(ChangelogRuleName),
	}
}

type ChangelogRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*ChangelogRule)(nil)

func (r *ChangelogRule) Check(_ context.Context) {
	path := filepath.Join(r.module.GetPath(), "changelog.yaml")
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
