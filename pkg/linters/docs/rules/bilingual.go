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

func NewBilingualRule() *BilingualRule {
	return &BilingualRule{
		RuleMeta: pkg.RuleMeta{
			Name: BilingualRuleName,
		},
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

	// Try to find README files in multiple locations
	readmePaths := []struct {
		enPath   string
		ruPath   string
		location string
	}{
		{
			filepath.Join(modulePath, "README.md"),
			filepath.Join(modulePath, "README_RU.md"),
			"root",
		},
		{
			filepath.Join(modulePath, "docs", "README.md"),
			filepath.Join(modulePath, "docs", "README_RU.md"),
			"docs/",
		},
	}

	var enExists, ruExists bool
	var enPath, ruPath, location string

	for _, paths := range readmePaths {
		enCheck := false
		ruCheck := false

		if _, err := os.Stat(paths.enPath); err == nil {
			enCheck = true
		}
		if _, err := os.Stat(paths.ruPath); err == nil {
			ruCheck = true
		}

		if enCheck || ruCheck {
			enExists = enCheck
			ruExists = ruCheck
			enPath = paths.enPath
			ruPath = paths.ruPath
			location = paths.location
			break
		}
	}

	if enExists && !ruExists {
		filePath := "README_RU.md"
		if location == "docs/" {
			filePath = "docs/README_RU.md"
		}
		errorList.
			WithFilePath(filePath).
			Error("README_RU.md file is missing - documentation should be available in both languages")
	}

	if ruExists && !enExists {
		filePath := "README.md"
		if location == "docs/" {
			filePath = "docs/README.md"
		}
		errorList.
			WithFilePath(filePath).
			Error("README.md file is missing - documentation should be available in both languages")
	}

	if enExists {
		if info, err := os.Stat(enPath); err == nil && info.Size() == 0 {
			filePath := "README.md"
			if location == "docs/" {
				filePath = "docs/README.md"
			}
			errorList.
				WithFilePath(filePath).
				Error("README.md file is empty")
		}
	}

	if ruExists {
		if info, err := os.Stat(ruPath); err == nil && info.Size() == 0 {
			filePath := "README_RU.md"
			if location == "docs/" {
				filePath = "docs/README_RU.md"
			}
			errorList.
				WithFilePath(filePath).
				Error("README_RU.md file is empty")
		}
	}
}
