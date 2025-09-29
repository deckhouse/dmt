// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
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
	CyrillicInEnglishRuleName = "cyrillic-in-english"
)

var (
	cyrRe              = regexp.MustCompile(`[А-Яа-яЁё]+`)
	cyrPointerRe       = regexp.MustCompile(`[А-Яа-яЁё]`)
	cyrFillerRe        = regexp.MustCompile(`[^А-Яа-яЁё]`)
	russianDocRe       = regexp.MustCompile(`\.ru\.md$`)
	russianDocUpperRe  = regexp.MustCompile(`_RU\.md$`)
	markdownExtensions = []string{".md", ".markdown"}
)

func NewCyrillicInEnglishRule() *CyrillicInEnglishRule {
	return &CyrillicInEnglishRule{
		RuleMeta: pkg.RuleMeta{
			Name: CyrillicInEnglishRuleName,
		},
	}
}

type CyrillicInEnglishRule struct {
	pkg.RuleMeta
	pkg.PathRule
}

func (r *CyrillicInEnglishRule) CheckFiles(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	modulePath := m.GetPath()
	if modulePath == "" {
		return
	}

	docsPath := filepath.Join(modulePath, "docs")
	files := fsutils.GetFiles(docsPath, false, fsutils.FilterFileByExtensions(markdownExtensions...))

	for _, fileName := range files {
		relFromModule := fsutils.Rel(modulePath, fileName)
		// only consider top-level docs/* files
		if filepath.Dir(relFromModule) != "docs" {
			continue
		}
		r.checkFile(m, fileName, errorList)
	}
}

func (r *CyrillicInEnglishRule) checkFile(m pkg.Module, fileName string, errorList *errors.LintRuleErrorsList) {
	relPath := fsutils.Rel(m.GetPath(), fileName)

	if !r.Enabled(relPath) {
		return
	}

	if russianDocRe.MatchString(fileName) {
		return
	}

	// TODO: Delete it after renaming to .ru.md view
	if russianDocUpperRe.MatchString(fileName) {
		return
	}

	lines, err := getFileContent(fileName)
	if err != nil {
		errorList.WithFilePath(relPath).WithValue(err.Error()).Error("failed to read file")
		return
	}

	cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
	if hasCyr {
		errorList.
			WithFilePath(relPath).
			WithValue(cyrMsg).
			Error("English documentation contains cyrillic characters")
	}
}

func getFileContent(filename string) ([]string, error) {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	sliceData := strings.Split(string(fileBytes), "\n")
	return sliceData, nil
}

func checkCyrillicLettersInString(line string) (string, bool) {
	if !cyrRe.MatchString(line) {
		return "", false
	}

	line = strings.TrimSpace(line)

	cursor := cyrFillerRe.ReplaceAllString(line, "-")
	cursor = cyrPointerRe.ReplaceAllString(cursor, "^")
	cursor = strings.TrimRight(cursor, "-")

	return line + "\n" + cursor, true
}

func checkCyrillicLettersInArray(lines []string) (string, bool) {
	res := make([]string, 0)

	hasCyr := false
	for i, line := range lines {
		msg, has := checkCyrillicLettersInString(line)
		if has {
			hasCyr = true
			res = append(res, fmt.Sprintf("Line %d: %s", i+1, msg))
		}
	}

	return strings.Join(res, "\n"), hasCyr
}
