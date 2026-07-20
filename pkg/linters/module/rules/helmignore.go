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
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/ignore"
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

// moduleTemplateExclude is the set of files and directories that belong to
// the Deckhouse module and therefore should NOT be listed in .helmignore.
// These are either required by Helm for rendering or read by Deckhouse
// directly from the module filesystem.
var moduleTemplateExclude = map[string]bool{
	// Required by Helm for chart rendering
	"templates":   true,
	"charts":      true,
	"monitoring":  true,
	"Chart.yaml":  true,
	"values.yaml": true,

	// Read by Deckhouse directly, not part of the Helm chart
	".namespace": true,
	"rbac.yaml":  true,
}

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

	// Validate that all module root files/dirs (except module-template entries)
	// are covered by .helmignore patterns.
	r.checkModuleRootCoverage(modulePath, raw, errorList)
}

// checkModuleRootCoverage scans the module root for all files and directories
// and verifies that everything except the standard module-template entries
// (templates/, charts/, Chart.yaml, values.yaml) is covered by a pattern in
// .helmignore. Helm's own ignore.Rules are used for proper pattern matching
// (wildcards, negation, directory-only rules, etc.).
func (r *HelmignoreRule) checkModuleRootCoverage(modulePath string, raw []byte, errorList *errors.LintRuleErrorsList) {
	entries, err := os.ReadDir(modulePath)
	if err != nil {
		errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot read module directory: %s", err)

		return
	}

	// Parse .helmignore using Helm's own rules engine.
	rules, err := ignore.Parse(bytes.NewReader(raw))
	if err != nil {
		errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot parse .helmignore: %s", err)

		return
	}

	rules.AddDefaults()

	for _, entry := range entries {
		name := entry.Name()

		// Skip .helmignore itself.
		if name == helmignoreFile {
			continue
		}

		// Skip entries that are part of the standard module template and
		// should NOT be ignored by Helm.
		if moduleTemplateExclude[name] {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			errorList.WithFilePath(helmignoreFile).
				Errorf("Cannot stat '%s': %s", name, err)

			continue
		}

		// Use Helm's ignore rules: if the entry is NOT ignored, it would be
		// included in the Helm chart — which we don't want for non-template
		// files/dirs.
		if rules.Ignore(name, info) {
			continue
		}

		entryType := "File"
		if entry.IsDir() {
			entryType = "Directory"
			name += "/"
		}

		errorList.WithFilePath(helmignoreFile).
			Warnf("%s '%s' is not listed in .helmignore", entryType, name)
	}
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
