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
	BilingualRuleName = "bilingual"
)

func NewBilingualRule(excludeModuleRules []pkg.StringRuleExclude, disable bool) *BilingualRule {
	return &BilingualRule{
		RuleMeta: pkg.RuleMeta{
			Name: BilingualRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules: excludeModuleRules,
		},
		disable: disable,
	}
}

type BilingualRule struct {
	pkg.RuleMeta
	pkg.PathRule
	disable bool
}

func (r *BilingualRule) CheckBilingual(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if r.disable {
		return
	}

	if !r.Enabled(m.GetName()) {
		return
	}

	modulePath := m.GetPath()
	readmeEnPath := filepath.Join(modulePath, "README.md")
	readmeRuPath := filepath.Join(modulePath, "README_RU.md")

	enExists := false
	if _, err := os.Stat(readmeEnPath); err == nil {
		enExists = true
	}

	ruExists := false
	if _, err := os.Stat(readmeRuPath); err == nil {
		ruExists = true
	}

	if enExists && !ruExists {
		errorList.
			WithFilePath("README_RU.md").
			Error("README_RU.md file is missing - documentation should be available in both languages")
	}

	if ruExists && !enExists {
		errorList.
			WithFilePath("README.md").
			Error("README.md file is missing - documentation should be available in both languages")
	}

	if enExists {
		if info, err := os.Stat(readmeEnPath); err == nil && info.Size() == 0 {
			errorList.
				WithFilePath("README.md").
				Error("README.md file is empty")
		}
	}

	if ruExists {
		if info, err := os.Stat(readmeRuPath); err == nil && info.Size() == 0 {
			errorList.
				WithFilePath("README_RU.md").
				Error("README_RU.md file is empty")
		}
	}
}
