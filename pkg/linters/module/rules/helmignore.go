/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rules

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	HelmignoreRuleName = "helmignore"
	helmignoreFile     = ".helmignore"

	// Helm template directory that should not be excluded
	helmTemplatesDir = "templates/"
	// Helm chart metadata file that should not be excluded
	helmChartYaml = "Chart.yaml"
)

// HelmignoreRule validates the .helmignore file itself: that it exists, says
// something, and that its patterns are well formed.
func NewHelmignoreRule(disable bool,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *HelmignoreRule {
	return &HelmignoreRule{
		RuleMeta: pkg.RuleMeta{
			Name: HelmignoreRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
		module:    m,
		errorList: errorList.WithRule(HelmignoreRuleName),
	}
}

type HelmignoreRule struct {
	pkg.RuleMeta
	pkg.BoolRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*HelmignoreRule)(nil)

func (r *HelmignoreRule) Check(_ context.Context) {
	modulePath := r.module.GetPath()
	errorList := r.errorList

	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	helmignorePath := filepath.Join(modulePath, helmignoreFile)

	// Check if .helmignore file exists
	_, err := os.Stat(helmignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			errorList.WithFilePath(helmignoreFile).
				Error("File .helmignore is required in module root")

			return
		}

		errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot stat .helmignore file: %s", err)

		return
	}

	// Read .helmignore content (raw bytes for helm ignore parser)
	raw, err := os.ReadFile(helmignorePath)
	if err != nil {
		errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot read .helmignore file: %s", err)

		return
	}

	// Parse non-comment, non-empty lines for pattern validation
	var lines []string

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	// Check if file is empty
	if len(lines) == 0 {
		errorList.WithFilePath(helmignoreFile).
			Error("File .helmignore is empty or contains only comments")

		return
	}

	// Validate patterns
	validatePatterns(lines, errorList)
}

func validatePatterns(patterns []string, errorList *errors.LintRuleErrorsList) {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		// Check for common invalid patterns
		if strings.Contains(pattern, " ") && !strings.HasPrefix(pattern, "#") {
			errorList.WithFilePath(helmignoreFile).
				Errorf("Pattern contains spaces without quotes: %q", pattern)
		}

		// Check for patterns that might be too broad
		if pattern == "*" || pattern == "**" {
			errorList.WithFilePath(helmignoreFile).
				Errorf("Pattern is too broad and will exclude everything: %q", pattern)
		}

		// Check for patterns that might exclude Helm templates
		if strings.Contains(pattern, helmTemplatesDir) && !strings.HasPrefix(pattern, "!") {
			errorList.WithFilePath(helmignoreFile).
				Errorf("Pattern might exclude Helm templates: %q", pattern)
		}

		// Check for patterns that might exclude Chart.yaml
		if strings.Contains(pattern, helmChartYaml) && !strings.HasPrefix(pattern, "!") {
			errorList.WithFilePath(helmignoreFile).
				Errorf("Pattern might exclude Chart.yaml: %q", pattern)
		}
	}
}
