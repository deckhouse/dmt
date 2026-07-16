// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ReadmeRuleName = "readme"
)

func NewReadmeRule() *ReadmeRule {
	return &ReadmeRule{
		RuleMeta: pkg.RuleMeta{
			Name: ReadmeRuleName,
		},
	}
}

type ReadmeRule struct {
	pkg.RuleMeta
	pkg.PathRule
}

func (r *ReadmeRule) CheckReadmeRemote(path string, errorList *errors.LintRuleErrorsList) {
	r.checkReadme(path, errorList)
}

func (r *ReadmeRule) CheckReadme(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	if !r.Enabled(m.GetName()) {
		return
	}

	r.checkReadme(m.GetPath(), errorList)
}

func (r *ReadmeRule) checkReadme(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

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
