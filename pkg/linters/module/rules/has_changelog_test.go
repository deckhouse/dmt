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

func TestHasChangelogRule(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		write   bool
		want    string
	}{
		{
			name:    "a changelog the module wrote",
			write:   true,
			content: "features:\n  General:\n    - a feature\n",
		},
		{
			name: "no changelog at all",
			want: "changelog.yaml file is missing",
		},
		{
			name:  "an empty changelog",
			write: true,
			want:  "changelog.yaml file is empty",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.write {
				require.NoError(t, os.WriteFile(
					filepath.Join(root, ChangelogFilename), []byte(tc.content), DefaultFilePerm))
			}

			errorList := errors.NewLintRuleErrorsList()
			NewHasChangelogRule(moduleAt(t, root), errorList).Check(context.Background())

			errs := errorList.GetErrors()
			if tc.want == "" {
				assert.Empty(t, errs)

				return
			}

			require.Len(t, errs, 1)
			assert.Equal(t, tc.want, errs[0].Text)
		})
	}
}
