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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertURLToModuleName(t *testing.T) {
	for _, tt := range []struct {
		name     string
		repoURL  string
		expected string
	}{
		{name: "SSH URL with .git suffix", repoURL: "git@github.com:deckhouse/test-module.git", expected: "test-module"},
		{name: "HTTPS URL with .git suffix", repoURL: "https://github.com/deckhouse/test-module.git", expected: "test-module"},
		{name: "HTTPS URL without .git suffix", repoURL: "https://github.com/deckhouse/test-module", expected: "test-module"},
		{name: "SSH URL without .git suffix", repoURL: "git@github.com:deckhouse/test-module", expected: "test-module"},
		{name: "complex path", repoURL: "https://github.com/deckhouse/ee/modules/test-module.git", expected: "test-module"},
		{name: "empty URL", repoURL: "", expected: ""},
		{name: "URL with trailing slash", repoURL: "https://github.com/deckhouse/test-module/", expected: ""},
		{name: "single component URL", repoURL: "test-module", expected: "test-module"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, convertURLToModuleName(tt.repoURL))
		})
	}
}

func TestGetGitConfigFile(t *testing.T) {
	t.Run("git config exists in current directory", func(t *testing.T) {
		dir := t.TempDir()
		writeGitConfig(t, dir, "test")

		assert.Equal(t, filepath.Join(dir, ".git", "config"), getGitConfigFile(dir))
	})

	t.Run("git config exists in parent directory", func(t *testing.T) {
		parent := t.TempDir()
		writeGitConfig(t, parent, "test")

		sub := filepath.Join(parent, "subdir")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		assert.Equal(t, filepath.Join(parent, ".git", "config"), getGitConfigFile(sub))
	})

	t.Run("no git config found", func(t *testing.T) {
		assert.Empty(t, getGitConfigFile(t.TempDir()))
	})

	t.Run("git directory exists but no config file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))

		assert.Empty(t, getGitConfigFile(dir))
	})
}

func TestRepositoryOriginURL(t *testing.T) {
	for _, tt := range []struct {
		name      string
		gitConfig string
		noGitDir  bool
		expected  string
	}{
		{
			name:      "SSH URL",
			gitConfig: "[remote \"origin\"]\n\turl = git@github.com:deckhouse/test-module.git",
			expected:  "git@github.com:deckhouse/test-module.git",
		},
		{
			name:      "HTTPS URL",
			gitConfig: "[remote \"origin\"]\n\turl = https://github.com/deckhouse/test-module.git",
			expected:  "https://github.com/deckhouse/test-module.git",
		},
		{
			// Not a URL at all — reading the config is this function's job, judging
			// what it holds is the caller's.
			name:      "origin that is not a URL",
			gitConfig: "[remote \"origin\"]\n\turl = invalid-url",
			expected:  "invalid-url",
		},
		{
			name:      "no origin section",
			gitConfig: "[core]\n\tbare = false",
			expected:  "",
		},
		{
			name:     "no git directory",
			noGitDir: true,
			expected: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.noGitDir {
				writeGitConfig(t, dir, tt.gitConfig)
			}

			assert.Equal(t, tt.expected, RepositoryOriginURL(dir))
		})
	}
}

func TestIsDeckhouseRepo(t *testing.T) {
	for _, tt := range []struct {
		name      string
		originURL string
		expected  bool
	}{
		{name: "the monorepo itself", originURL: "git@github.com:deckhouse/deckhouse.git", expected: true},
		{name: "a fork of the monorepo", originURL: "https://github.com/someone/deckhouse.git", expected: true},
		{name: "a test monorepo from the ignore list", originURL: "git@github.com:deckhouse/deckhouse-test-1.git", expected: true},
		{name: "a standalone module in the same org", originURL: "git@github.com:deckhouse/log-shipper.git", expected: false},
		{name: "no origin at all", originURL: "", expected: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.originURL != "" {
				writeGitConfig(t, dir, "[remote \"origin\"]\n\turl = "+tt.originURL)
			}

			assert.Equal(t, tt.expected, IsDeckhouseRepo(dir))
		})
	}
}

func writeGitConfig(t *testing.T, dir, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(content), 0o600))
}
