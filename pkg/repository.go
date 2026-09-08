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

package pkg

import (
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/fsutils"
)

// TODO: THINK ABOUT HOW TO ENDURE
var IgnoreDeckhouseReposList = []string{"deckhouse", "deckhouse-test-1", "deckhouse-test-2"}

// RepositoryOriginURL returns the `remote "origin"` URL of the git checkout dir sits in,
// or "" when there is no checkout, no origin, or the config cannot be read. Every caller
// that needs to know which repository a module was linted from starts here.
func RepositoryOriginURL(dir string) string {
	configFile := getGitConfigFile(dir)
	if configFile == "" {
		return ""
	}

	cfg, err := ini.Load(configFile)
	if err != nil {
		log.Error("Failed to load config file", log.Err(err))
		return ""
	}

	sec, err := cfg.GetSection("remote \"origin\"")
	if err != nil {
		log.Error("Failed to get remote origin", log.Err(err))
		return ""
	}

	return sec.Key("url").String()
}

// IsDeckhouseRepo reports a module linted from inside the Deckhouse monorepo, by the name
// of the repository it sits in. Such a module is built and released by the platform rather
// than on its own, so the rules that describe what a standalone module publishes — and the
// requirements only a standalone module can state — do not apply to it.
func IsDeckhouseRepo(dir string) bool {
	return slices.Contains(IgnoreDeckhouseReposList, convertURLToModuleName(RepositoryOriginURL(dir)))
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
