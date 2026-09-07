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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestLeftoversRule(t *testing.T) {
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
			name:          "build directory the patterns should have stripped",
			helmignore:    "images/\n",
			dirs:          []string{"images", "templates"},
			wantFilePaths: []string{"images"},
		},
		{
			name:       "metadata Deckhouse reads off the filesystem is not a leftover",
			helmignore: "openapi/\ncrds/\ndocs/\nhooks/\nmodule.yaml\n",
			dirs:       []string{"openapi", "crds", "docs", "hooks", "templates"},
			files:      []string{"module.yaml"},
		},
		{
			name:       "an entry no pattern excludes is none of our business",
			helmignore: "images/\n",
			dirs:       []string{"templates", "tools"},
		},
		{
			// AddDefaults only contributes `templates/.?*`, which no root entry matches,
			// so a dotfile is a finding when the module's own patterns name it and not
			// otherwise.
			name:          "a dotfile the module excluded itself",
			helmignore:    "images/\n.git/\n",
			dirs:          []string{".git", "templates"},
			wantFilePaths: []string{".git"},
		},
		{
			name:       "a dotfile no pattern names is not our finding",
			helmignore: "images/\n",
			dirs:       []string{".git", "templates"},
		},
		{
			name:          "several leftovers are several findings",
			helmignore:    "images/\nwerf.yaml\nhack/\n",
			dirs:          []string{"images", "hack", "templates"},
			files:         []string{"werf.yaml"},
			wantFilePaths: []string{"images", "hack", "werf.yaml"},
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
			NewHelmignoreLeftoversRule(moduleAt(t, root), errorList).Check(t.Context())

			got := make([]string, 0, len(errorList.GetErrors()))
			for _, e := range errorList.GetErrors() {
				got = append(got, e.FilePath)
			}

			assert.ElementsMatch(t, tt.wantFilePaths, got)
		})
	}
}

// TestLeftoversRuleSkipsHelmignoreItself pins the entry a broad pattern would otherwise
// turn into a finding against itself: bundle-layout requires .helmignore in the package
// root, so matching its own `.*` must not report it.
func TestLeftoversRuleSkipsHelmignoreItself(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".helmignore"), []byte(".*\n"), 0600))

	errorList := errors.NewLintRuleErrorsList()
	NewHelmignoreLeftoversRule(moduleAt(t, root), errorList).Check(t.Context())

	assert.Empty(t, errorList.GetErrors())
}
