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

	cfg := gomarkdownlint.ConfigFromMap(deckhouseMarkdownlintConfig())

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

func deckhouseMarkdownlintConfig() map[string]any {
	return map[string]any{
		// MD002/first-heading-h1/first-header-h1 - First heading should be a top-level heading (deprecated)
		"MD002": false,

		// MD004/ul-style - Unordered list style
		"MD004": false,

		// MD013/line-length - Line length
		"MD013": map[string]any{
			"line_length":            1000,  // Number of characters
			"heading_line_length":    128,   // Number of characters for headings
			"code_block_line_length": 400,   // Number of characters for code blocks
			"code_blocks":            true,  // Include code blocks
			"tables":                 true,  // Include tables
			"headings":               true,  // Include headings
			"headers":                true,  // Include headings (deprecated alias)
			"strict":                 false, // Strict length checking
			"stern":                  false, // Stern length checking
		},

		// MD022/blanks-around-headings/blanks-around-headers - Headings should be surrounded by blank lines
		"MD022": map[string]any{
			"lines_above": 1, // Blank lines above heading
			"lines_below": 1, // Blank lines below heading
		},

		// MD024/no-duplicate-heading/no-duplicate-header - Multiple headings with the same content
		"MD024": map[string]any{
			"siblings_only": true, // Only check sibling headings
		},

		// MD026/no-trailing-punctuation - Trailing punctuation in heading
		"MD026": map[string]any{
			"punctuation": ".,;:!。，；：！", // Punctuation characters
		},

		// MD029/ol-prefix - Ordered list item prefix
		"MD029": map[string]any{
			"style": "one_or_ordered", // List style
		},

		// MD033/no-inline-html - Inline HTML
		"MD033": false,

		// MD032/blanks-around-lists - Lists should be surrounded by blank lines
		"MD032": false,

		// MD041/first-line-heading/first-line-h1 - First line in a file should be a top-level heading
		"MD041": map[string]any{
			"level":              1,                  // Heading level
			"front_matter_title": `^\s*title\s*[:=]`, // RegExp for matching title in front matter
		},

		// MD042/no-empty-links - No empty links
		"MD042": true,

		// MD043/required-headings/required-headers - Required heading structure
		"MD043": map[string]any{
			"headings": nil, // List of headings
			"headers":  nil, // List of headings (deprecated alias)
		},

		// MD044/proper-names - Proper names should have the correct capitalization
		"MD044": map[string]any{
			"names":       []string{}, // List of proper names
			"code_blocks": true,       // Include code blocks
		},

		// MD045/no-alt-text - Images should have alternate text (alt text)
		"MD045": true,

		// MD046/code-block-style - Code block style
		"MD046": map[string]any{
			"style": "consistent", // Block style
		},

		// MD047/single-trailing-newline - Files should end with a single newline character
		"MD047": true,

		// MD048/code-fence-style - Code fence style
		"MD048": map[string]any{
			"style": "consistent", // Code fence style
		},

		// MD049/emphasis-style - Emphasis style should be consistent
		"MD049": map[string]any{
			"style": "consistent", // Emphasis style should be consistent
		},

		// MD050/strong-style - Strong style should be consistent
		"MD050": map[string]any{
			"style": "consistent", // Strong style should be consistent
		},
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
