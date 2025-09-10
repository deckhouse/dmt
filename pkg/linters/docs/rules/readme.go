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

func NewReadmeRule(excludeModuleRules []pkg.StringRuleExclude) *ReadmeRule {
	return &ReadmeRule{
		RuleMeta: pkg.RuleMeta{
			Name: ReadmeRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules: excludeModuleRules,
		},
	}
}

type ReadmeRule struct {
	pkg.RuleMeta
	pkg.PathRule
}

func (r *ReadmeRule) CheckReadme(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !r.Enabled(m.GetName()) {
		return
	}

	modulePath := m.GetPath()
	readmePath := filepath.Join(modulePath, "README.md")

	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		errorList.
			WithFilePath("README.md").
			Error("README.md file is missing in module")
		return
	}

	info, err := os.Stat(readmePath)
	if err != nil {
		errorList.
			WithFilePath("README.md").
			WithValue(err.Error()).
			Error("failed to check README.md file")
		return
	}

	if info.Size() == 0 {
		errorList.
			WithFilePath("README.md").
			Error("README.md file is empty")
	}
}
