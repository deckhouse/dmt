// Copyright 2026 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	NoLangKeyRuleName = "no-lang-key"
)

var (
	frontMatterDelimiter = regexp.MustCompile(`^---\s*$`)
	langKeyRe            = regexp.MustCompile(`(?m)^lang:\s`)
)

func NewNoLangKeyRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *NoLangKeyRule {
	return &NoLangKeyRule{
		RuleMeta: pkg.RuleMeta{
			Name: NoLangKeyRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(NoLangKeyRuleName),
	}
}

type NoLangKeyRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*NoLangKeyRule)(nil)

func (r *NoLangKeyRule) Check(_ context.Context) {
	modulePath := r.module.GetPath()
	if modulePath == "" {
		return
	}

	docsPath := filepath.Join(modulePath, "docs")
	files := fsutils.GetFiles(docsPath, false, fsutils.FilterFileByExtensions(".md"))

	for _, fileName := range files {
		relFromModule := fsutils.Rel(modulePath, fileName)
		if filepath.Dir(relFromModule) != "docs" {
			continue
		}

		r.checkFile(fileName)
	}
}

func (r *NoLangKeyRule) checkFile(fileName string) {
	errorList := r.errorList
	relPath := fsutils.Rel(r.module.GetPath(), fileName)

	if !r.Enabled(relPath) {
		return
	}

	content, err := os.ReadFile(fileName)
	if err != nil {
		errorList.WithFilePath(relPath).WithValue(err.Error()).Error("failed to read file")
		return
	}

	frontMatter := extractFrontMatter(string(content))
	if frontMatter == "" {
		return
	}

	if langKeyRe.MatchString(frontMatter) {
		lineNum := findLangKeyLine(string(content))
		msg := fmt.Sprintf("Line %d: front matter contains 'lang' key which should be removed", lineNum)
		errorList.
			WithFilePath(relPath).
			WithValue(msg).
			Error("Documentation contains 'lang' key in front matter; this field should be removed")
	}
}

// extractFrontMatter returns the YAML front matter content between the first pair of "---" delimiters.
// Returns an empty string if no valid front matter is found.
func extractFrontMatter(content string) string {
	lines := strings.Split(content, "\n")

	startIdx := -1
	endIdx := -1

	for i, line := range lines {
		if frontMatterDelimiter.MatchString(line) {
			if startIdx == -1 {
				startIdx = i
			} else {
				endIdx = i
				break
			}
		}
	}

	if startIdx == -1 || endIdx == -1 {
		return ""
	}

	return strings.Join(lines[startIdx+1:endIdx], "\n")
}

// findLangKeyLine returns the 1-based line number where the 'lang:' key appears in the file content.
func findLangKeyLine(content string) int {
	lines := strings.Split(content, "\n")
	langLineRe := regexp.MustCompile(`^lang:\s`)

	for i, line := range lines {
		if langLineRe.MatchString(line) {
			return i + 1
		}
	}

	return 0
}
