// Copyright 2026 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	gomarkdownlint "github.com/ldmonster/go-markdownlint"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	MarkdownName = "markdownlint"
)

func NewMarkdownRule() *MarkdownRule {
	return &MarkdownRule{
		RuleMeta: pkg.RuleMeta{
			Name: MarkdownName,
		},
	}
}

type MarkdownRule struct {
	pkg.RuleMeta
	pkg.PathRule
}

func (r *MarkdownRule) Run(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !r.Enabled(m.GetName()) {
		return
	}

	modulePath := m.GetPath()

	mdFiles, err := collectMarkdownFiles(modulePath)
	if err != nil {
		errorList.
			WithFilePath(modulePath).
			WithValue(err.Error()).
			Errorf("failed to collect markdown files: %s", err)

		return
	}

	if len(mdFiles) == 0 {
		return
	}

	cfg := gomarkdownlint.ConfigFromMap(map[string]any{
		"default": true,
	})

	results, err := gomarkdownlint.LintFiles(context.Background(), mdFiles, cfg)
	if err != nil {
		errorList.
			WithFilePath(modulePath).
			WithValue(err.Error()).
			Errorf("markdownlint failed: %s", err)

		return
	}

	for file, errs := range results {
		for _, mdErr := range errs {
			errorList.
				WithFilePath(file).
				WithLineNumber(mdErr.LineNumber).
				Errorf("%s %s", strings.Join(mdErr.RuleNames, "/"), mdErr.RuleDescription)
		}
	}
}

func collectMarkdownFiles(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}
