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
	if split := strings.Split(repositoryULR, "@"); len(split) > 1 {
		repositoryULR = split[1]
	}
	repositoryULR = strings.TrimSuffix(repositoryULR, ".git")

	return repositoryULR
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
