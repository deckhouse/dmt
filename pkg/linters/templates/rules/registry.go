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

	"gopkg.in/ini.v1"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	RegistryRuleName = "registry"
)

func NewRegistryRule() *RegistryRule {
	return &RegistryRule{
		RuleMeta: pkg.RuleMeta{
			Name: RegistryRuleName,
		},
	}
}

type RegistryRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

// CheckRegistrySecret checks module registry secret for the module.
func (r *RegistryRule) CheckRegistrySecret(md *module.Module, errorList *errors.LintRuleErrorsList) {
	registryFile := fsutils.GetFiles(md.GetPath(), false, fsutils.FilterFileByNames("registry-secret.yaml"))
	if len(registryFile) == 0 {
		return
	}

	moduleName := getModuleNameFromRepository(md.GetPath())
	if moduleName == "deckhouse" {
		// Skip registry secret check for deckhouse module
		return
	}

	errorList = errorList.WithRule(r.GetName())

	// Read the file content
	fileContent, err := os.ReadFile(registryFile[0])
	if err != nil {
		errorList.Errorf("failed to read registry secret file: %v", err)
		return
	}

	// Check if file contains forbidden string
	if strings.Contains(string(fileContent), ".Values.global.modulesImages") {
		errorList.Error("registry-secret.yaml file should not contain .Values.global.modulesImages")
	}
}

func getModuleNameFromRepository(dir string) string {
	configFile := getGitConfigFile(dir)
	if configFile == "" {
		return ""
	}

	cfg, err := ini.Load(configFile)
	if err != nil {
		logger.ErrorF("Failed to load config file: %v", err)
		return ""
	}

	sec, err := cfg.GetSection("remote \"origin\"")
	if err != nil {
		logger.ErrorF("Failed to get remote \"origin\": %v", err)
		return ""
	}

	repositoryURL := sec.Key("url").String()
	return convertURLToModuleName(repositoryURL)
}

func getGitConfigFile(dir string) string {
	for {
		if fsutils.IsDir(filepath.Join(dir, ".git")) &&
			fsutils.IsFile(filepath.Join(dir, ".git", "config")) {
			return filepath.Join(dir, ".git", "config")
		}
		parent := filepath.Dir(dir)
		if dir == parent || parent == "" {
			break
		}

		dir = parent
	}

	return ""
}

// convertURLToModuleName converts a repository URL to a module name.
// It handles both SSH and HTTPS formats.
// Examples:
// git@github.com:deckhouse/dmt.git
// https://github.com/deckhouse/dmt
// It returns the last part of the URL as the module name.
// For example, for the URL "git@github.com:deckhouse/dmt.git", it will return "dmt".
func convertURLToModuleName(repoURL string) string {
	// Remove the protocol part if it exists
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "git@")

	// Remove the ".git" suffix if it exists
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Split by '/' and return the last part
	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[len(parts)-1]
}
