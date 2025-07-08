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
	"os"
	"path/filepath"
	"strings"

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

func NewHelmignoreRule(disable bool) *HelmignoreRule {
	return &HelmignoreRule{
		RuleMeta: pkg.RuleMeta{
			Name: HelmignoreRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
	}
}

type HelmignoreRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

func (r *HelmignoreRule) CheckHelmignore(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !r.Enabled() {
		// TODO: add metrics
		return
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

	// Read and parse .helmignore file
	file, err := os.Open(helmignorePath)
	if err != nil {
		errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot open .helmignore file: %s", err)
		return
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		errorList.WithFilePath(helmignoreFile).
			Errorf("Error reading .helmignore file: %s", err)
		return
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
