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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestChangelogYAMLRule(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		write   bool
		wantErr bool
	}{
		{
			name:    "a changelog the builder wrote",
			write:   true,
			content: "log-shipper:\n  features:\n    - summary: a feature\n      pull_request: https://example.invalid/1\n",
		},
		{
			// Syntax only, so a file that parses but holds nothing recognisable is
			// deliberately clean here.
			name:    "valid YAML of an unexpected shape",
			write:   true,
			content: "just a string\n",
		},
		{
			name:    "empty file",
			write:   true,
			content: "",
		},
		{
			// has-changelog owns presence; this rule must stay quiet, which is what
			// lets a scope whose image carries no changelog ask for it.
			name:  "no changelog at all",
			write: false,
		},
		{
			name:    "broken YAML",
			write:   true,
			content: "log-shipper:\n  features:\n   - summary: a feature\n  \tpull_request: 1\n",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.write {
				require.NoError(t, os.WriteFile(
					filepath.Join(root, ChangelogFilename), []byte(tc.content), 0o600))
			}

			errorList := errors.NewLintRuleErrorsList()
			NewChangelogValidRule(moduleAt(t, root), errorList).Check(context.Background())

			errs := errorList.GetErrors()
			if !tc.wantErr {
				assert.Empty(t, errs)

				return
			}

			require.Len(t, errs, 1)
			assert.Contains(t, errs[0].Text, "invalid YAML in changelog.yaml")
		})
	}
}

// A module with no path is what a scope hands a rule when nothing was unpacked; the
// rule must not read the working directory instead.
func TestChangelogYAMLRuleWithoutAPath(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()
	NewChangelogValidRule(moduleAt(t, ""), errorList).Check(context.Background())

	assert.Empty(t, errorList.GetErrors())
}
