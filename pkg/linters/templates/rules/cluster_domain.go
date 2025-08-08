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
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ClusterDomainRuleName  = "cluster-domain"
	clusterLocalSubstring  = "cluster.local"
	recommendedReplacement = ".Values.global.clusterConfiguration.clusterDomain"
)

type ClusterDomainRule struct {
	pkg.RuleMeta
}

func NewClusterDomainRule() *ClusterDomainRule {
	return &ClusterDomainRule{
		RuleMeta: pkg.RuleMeta{
			Name: ClusterDomainRuleName,
		},
	}
}

type iModuleWithPath interface {
	GetName() string
	GetPath() string
}

func (r *ClusterDomainRule) ValidateClusterDomainInTemplates(m iModuleWithPath, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	templatesPath := filepath.Join(m.GetPath(), "templates")

	// Check if templates directory exists
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	// Get all files in templates directory
	files := fsutils.GetFiles(templatesPath, true)

	for _, filePath := range files {
		// Skip non-template files
		if !isTemplateFile(filePath) {
			continue
		}

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

		// Check for cluster.local substring
		if strings.Contains(string(content), clusterLocalSubstring) {
			errorList.WithObjectID(relPath).
				Errorf("File contains hardcoded 'cluster.local' substring. Use '%s' instead for dynamic cluster domain configuration.", recommendedReplacement)
		}
	}
}

func isTemplateFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".yaml" || ext == ".yml" || ext == ".tpl" || ext == ".tpl.yaml" || ext == ".tpl.yml"
}
