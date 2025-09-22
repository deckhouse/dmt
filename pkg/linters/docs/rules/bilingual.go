// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
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

	docsPath := filepath.Join(modulePath, "docs")
	if _, err := os.Stat(docsPath); err != nil {
		return
	}

	files := fsutils.GetFiles(docsPath, false, fsutils.FilterFileByExtensions(".md"))

	fileSet := make(map[string]struct{}, len(files))
	for _, f := range files {
		rel := fsutils.Rel(modulePath, f)
		fileSet[rel] = struct{}{}
	}

	for rel := range fileSet {
		if !strings.HasPrefix(rel, "docs/") {
			continue
		}
		if !strings.HasSuffix(rel, ".md") || strings.HasSuffix(rel, ".ru.md") {
			continue
		}

		base := strings.TrimSuffix(rel, ".md")
		ruRel := base + ".ru.md"
		if _, ok := fileSet[ruRel]; !ok {
			errorList.
				WithFilePath(rel).
				Error("Russian counterpart is missing: need to create a matching .ru.md in docs/")
		}
	}
}
