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
	ReadmeRuleName = "readme"
)

func NewReadmeRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *ReadmeRule {
	return &ReadmeRule{
		RuleMeta: pkg.RuleMeta{
			Name: ReadmeRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(ReadmeRuleName),
	}
}

type ReadmeRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*ReadmeRule)(nil)

func (r *ReadmeRule) Check(_ context.Context) {
	m, errorList := r.module, r.errorList

	if !r.Enabled(m.GetName()) {
		return
	}

	modulePath := m.GetPath()
	path := filepath.Join(modulePath, "docs", "README.md")

	if _, err := os.Stat(path); err != nil {
		errorList.
			WithFilePath(path).
			Error("README.md file is missing in docs/ directory")

		return
	}

	info, err := os.Stat(path)
	if err != nil {
		errorList.
			WithFilePath(path).
			WithValue(err.Error()).
			Error("failed to check README.md file")

		return
	}

	if info.Size() == 0 {
		errorList.
			WithFilePath(path).
			Error("README.md file is empty")
	}
}
