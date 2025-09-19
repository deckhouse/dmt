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

func (r *ReadmeRule) CheckReadme(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !r.Enabled(m.GetName()) {
		return
	}

	modulePath := m.GetPath()

	readmePaths := []struct {
		path     string
		location string
	}{
		{filepath.Join(modulePath, "README.md"), "README.md"},
		{filepath.Join(modulePath, "docs", "README.md"), "docs/README.md"},
	}

	var foundPath string
	var foundLocation string

	for _, readmeInfo := range readmePaths {
		if _, err := os.Stat(readmeInfo.path); err == nil {
			foundPath = readmeInfo.path
			foundLocation = readmeInfo.location
			break
		}
	}

	if foundPath == "" {
		errorList.
			WithFilePath("README.md").
			Error("README.md file is missing in module (checked root and docs/ directory)")
		return
	}

	info, err := os.Stat(foundPath)
	if err != nil {
		errorList.
			WithFilePath(foundLocation).
			WithValue(err.Error()).
			Error("failed to check README.md file")
		return
	}

	if info.Size() == 0 {
		errorList.
			WithFilePath(foundLocation).
			Error("README.md file is empty")
	}
}
