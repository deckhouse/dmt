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

	// relPath is what the finding names and path is what is read: for a remote scope
	// the module path is a temporary extraction directory, removed before findings
	// are printed, so reporting it would point at nothing.
	relPath := filepath.Join("docs", "README.md")
	path := filepath.Join(m.GetPath(), relPath)

	if _, err := os.Stat(path); err != nil {
		errorList.
			WithFilePath(relPath).
			Error("README.md file is missing in docs/ directory")

		return
	}

	info, err := os.Stat(path)
	if err != nil {
		errorList.
			WithFilePath(relPath).
			WithValue(err.Error()).
			Error("failed to check README.md file")

		return
	}

	if info.Size() == 0 {
		errorList.
			WithFilePath(relPath).
			Error("README.md file is empty")
	}
}
