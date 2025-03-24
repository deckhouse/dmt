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

package metrics

import (
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
)

func getRepositoryAddress(dir string) string {
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

	repositoryULR := sec.Key("url").String()
	return convertToHTTPS(repositoryULR)
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

func convertToHTTPS(repoURL string) string {
	if strings.HasPrefix(repoURL, "git@") {
		// Convert SSH format to HTTPS
		repoURL = strings.Replace(repoURL, ":", "/", 1)
		repoURL = strings.Replace(repoURL, "git@", "https://", 1)
		repoURL = strings.TrimSuffix(repoURL, ".git")
	}
	return repoURL
}
