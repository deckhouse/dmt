/*
Copyright 2026 Flant JSC

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
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	EnabledModulesRuleName = "enabled-modules"
)

var enabledModulesRe = regexp.MustCompile(`\.Values\.global\.enabledModules\s*\|\s*has\s+"([^"]*)"`)

type EnabledModulesRule struct {
	pkg.RuleMeta
}

func NewEnabledModulesRule() *EnabledModulesRule {
	return &EnabledModulesRule{
		RuleMeta: pkg.RuleMeta{
			Name: EnabledModulesRuleName,
		},
	}
}

func (r *EnabledModulesRule) CheckEnabledModules(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	templatesPath := filepath.Join(m.GetPath(), "templates")

	// Check if templates directory exists
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	// Get all files in templates directory
	files := fsutils.GetFiles(templatesPath, true, fsutils.FilterFileByExtensions(".yaml", ".yml", ".tpl"))

	for _, filePath := range files {
		// Get relative path for error reporting
		relPath, err := filepath.Rel(m.GetPath(), filePath)
		if err != nil {
			errorList.Errorf("Failed to get relative path for file %s: %v", filePath, err)
			continue
		}

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			errorList.Errorf("Failed to read file %s: %v", relPath, err)
			continue
		}

		// Find all matches of the pattern
		matches := enabledModulesRe.FindAllStringSubmatchIndex(string(content), -1)
		for _, match := range matches {
			// match[0]: start of full match, match[1]: end of full match
			// match[2]: start of capture group 1 (module name), match[3]: end of capture group 1
			matchStart := match[0]
			moduleName := string(content[match[2]:match[3]])

			line := strings.Count(string(content[:matchStart]), "\n") + 1

			errorList.WithRule(r.GetName()).
				WithFilePath(relPath).
				WithLineNumber(line).
				Errorf("Found usage of .Values.global.enabledModules | has \"%s\".\nConsider using (.Capabilities.APIVersions.Has \"group/version/Kind\") instead.", moduleName)
		}
	}
}
