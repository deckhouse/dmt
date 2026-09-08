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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestCoverageRule(t *testing.T) {
	tests := []struct {
		name          string
		helmignore    string // empty means no .helmignore at all
		dirs          []string
		files         []string
		wantFilePaths []string
	}{
		{
			name:       "no .helmignore is bundle-layout's finding, not ours",
			helmignore: "",
			dirs:       []string{"images"},
		},
		{
			name:       "everything non-chart is covered",
			helmignore: "hooks/\nopenapi/\ncrds/\ndocs/\nmodule.yaml\n",
			dirs:       []string{"hooks", "openapi", "crds", "docs", "templates", "charts"},
			files:      []string{"module.yaml", "Chart.yaml"},
		},
		{
			name:          "an uncovered directory is a finding",
			helmignore:    "hooks/\n",
			dirs:          []string{"hooks", "images", "templates"},
			wantFilePaths: []string{"images"},
		},
		{
			name:          "an uncovered file is a finding",
			helmignore:    "hooks/\n",
			dirs:          []string{"hooks", "templates"},
			files:         []string{"leftover.yaml", "werf.yaml"},
			wantFilePaths: []string{"leftover.yaml", "werf.yaml"},
		},
		{
			name:       "a directory covered without a trailing slash counts as covered",
			helmignore: "hooks\n",
			dirs:       []string{"hooks", "templates"},
		},
		{
			// helm applies a directory-only pattern to the entries inside the directory,
			// not to the directory itself, so the wildcard leaves it uncovered.
			name:          "images/* does not cover the directory itself",
			helmignore:    "images/*\n",
			dirs:          []string{"images", "templates"},
			wantFilePaths: []string{"images"},
		},
		{
			name:          "a negated pattern does not count as covered",
			helmignore:    "!images/\n",
			dirs:          []string{"images", "templates"},
			wantFilePaths: []string{"images"},
		},
		{
			name:       "chart material needs no pattern",
			helmignore: "hooks/\n",
			dirs:       []string{"templates", "charts", "monitoring"},
			files:      []string{"Chart.yaml", "values.yaml", "images_digests.json"},
		},
		{
			name:          "helm rejects double-star, and we say so once",
			helmignore:    "images/**\n",
			dirs:          []string{"images"},
			wantFilePaths: []string{".helmignore"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			for _, dir := range tt.dirs {
				require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0750))
			}

			for _, file := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(root, file), []byte("x"), 0600))
			}

			if tt.helmignore != "" {
				require.NoError(t, os.WriteFile(filepath.Join(root, ".helmignore"), []byte(tt.helmignore), 0600))
			}

			errorList := errors.NewLintRuleErrorsList()
			NewHelmignoreCoverageRule(moduleAt(t, root), errorList).Check(t.Context())

			got := make([]string, 0, len(errorList.GetErrors()))
			for _, e := range errorList.GetErrors() {
				got = append(got, e.FilePath)
			}

			assert.ElementsMatch(t, tt.wantFilePaths, got)
		})
	}
}

// TestCoverageRuleReportsAtWarn pins the severity: an uncovered file bloats the chart, it
// does not break it, so the finding must not fail a build at the default level.
func TestCoverageRuleReportsAtWarn(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".helmignore"), []byte("hooks/\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "leftover.yaml"), []byte("x"), 0600))

	errorList := errors.NewLintRuleErrorsList()
	NewHelmignoreCoverageRule(moduleAt(t, root), errorList).Check(t.Context())

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Equal(t, pkg.Warn, errs[0].Level)
}

// TestCoverageRuleSkipsHelmignoreItself pins the entry a broad pattern would otherwise
// leave uncovered against itself: bundle-layout requires .helmignore in the package root,
// and no .helmignore lists itself.
func TestCoverageRuleSkipsHelmignoreItself(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".helmignore"), []byte("hooks/\n"), 0600))

	errorList := errors.NewLintRuleErrorsList()
	NewHelmignoreCoverageRule(moduleAt(t, root), errorList).Check(t.Context())

	assert.Empty(t, errorList.GetErrors())
}
