// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	CyrillicInEnglishRuleName = "cyrillic-in-english"

	// maxCyrillicReportLines and maxCyrillicLineWidth bound how much of a file is
	// echoed back in a single finding, so a document full of Cyrillic cannot dump
	// megabytes into the log.
	maxCyrillicReportLines = 100
	maxCyrillicLineWidth   = 200
)

var (
	cyrRe              = regexp.MustCompile(`[А-Яа-яЁё]+`)
	cyrPointerRe       = regexp.MustCompile(`[А-Яа-яЁё]`)
	cyrFillerRe        = regexp.MustCompile(`[^А-Яа-яЁё]`)
	russianDocRe       = regexp.MustCompile(`\.ru\.md$`)
	russianDocUpperRe  = regexp.MustCompile(`(?i)_ru\.md$`)
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

	// sizeExcludes gates only the large-file size warning (not the Cyrillic
	// content check), so a file/directory can be excluded from the size check
	// alone.
	sizeExcludes pkg.PathRule
}

// WithFileSizeExcludes configures the files/directories excluded from the
// large-file size warning.
func (r *CyrillicInEnglishRule) WithFileSizeExcludes(files []pkg.StringRuleExclude, dirs []pkg.DirectoryRuleExclude) *CyrillicInEnglishRule {
	r.sizeExcludes = pkg.PathRule{ExcludeStringRules: files, ExcludeDirectoryRules: dirs}

	return r
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
		if fsutils.IsFileTooLarge(err) {
			// Too large to scan; report it as a warning unless the file or its
			// directory is excluded from the size check.
			if r.sizeExcludes.Enabled(relPath) {
				errorList.WithFilePath(relPath).
					Warnf("file is too large to check for Cyrillic letters and was skipped; exclude the file or its directory under documentation.exclude-rules.file-size to silence this warning")
			}

			return
		}

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
	fileBytes, err := fsutils.ReadFile(filename)
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

	// Bound the width of a single reported line so a very long minified line
	// cannot flood the log.
	if runes := []rune(line); len(runes) > maxCyrillicLineWidth {
		line = string(runes[:maxCyrillicLineWidth]) + "…"
	}

	cursor := cyrFillerRe.ReplaceAllString(line, "-")
	cursor = cyrPointerRe.ReplaceAllString(cursor, "^")
	cursor = strings.TrimRight(cursor, "-")

	return line + "\n" + cursor, true
}

func checkCyrillicLettersInArray(lines []string) (string, bool) {
	res := make([]string, 0)

	hasCyr := false
	truncated := 0

	for i, line := range lines {
		msg, has := checkCyrillicLettersInString(line)
		if !has {
			continue
		}

		hasCyr = true

		// Cap the number of reported lines so a document full of Cyrillic cannot
		// produce a multi-megabyte finding.
		if len(res) >= maxCyrillicReportLines {
			truncated++

			continue
		}

		res = append(res, fmt.Sprintf("Line %d: %s", i+1, msg))
	}

	out := strings.Join(res, "\n")
	if truncated > 0 {
		out += fmt.Sprintf("\n… and %d more line(s) with Cyrillic letters (truncated)", truncated)
	}

	return out, hasCyr
}
