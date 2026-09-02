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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestLayoutRules(t *testing.T) {
	releaseFiles := []string{"module.yaml", "version.json", "changelog.yaml"}
	// The bundle fixture is the root of a real published bundle, not a copy of the
	// rule's own list — deriving it from the rule would only prove the rule agrees
	// with itself. Eight CE bundles (sds-node-configurator, sds-local-volume,
	// sds-replicated-volume, csi-nfs, console, commander-agent, observability,
	// secrets-store-integration) all carry these, and differ only in the optional
	// crds/, hooks/, monitoring/ and .werf/.
	bundleFiles := []string{".helmignore", "Chart.yaml", "images_digests.json", "module.yaml"}
	bundleDirs := []string{"charts", "docs", "openapi", "templates"}

	t.Run("release layout is complete", func(t *testing.T) {
		root := layoutAt(t, releaseFiles, nil)

		assert.Empty(t, checkLayout(t, NewReleaseLayoutRule, root))
	})

	t.Run("bundle layout is complete", func(t *testing.T) {
		root := layoutAt(t, bundleFiles, bundleDirs)

		assert.Empty(t, checkLayout(t, NewBundleLayoutRule, root))
	})

	t.Run("a missing file is one finding", func(t *testing.T) {
		root := layoutAt(t, releaseFiles, nil)
		require.NoError(t, os.Remove(filepath.Join(root, "version.json")))

		errs := checkLayout(t, NewReleaseLayoutRule, root)

		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Text, "version.json file is missing")
	})

	t.Run("the wrong kind is reported as such", func(t *testing.T) {
		// docs is a directory in a bundle; a file by that name is not the same thing.
		root := layoutAt(t, append(bundleFiles, "docs"), []string{"charts", "openapi", "templates"})

		errs := checkLayout(t, NewBundleLayoutRule, root)

		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Text, "docs must be a directory")
	})
}

func checkLayout(t *testing.T, newRule func(pkg.Module, *errors.LintRuleErrorsList) *LayoutRule, root string) []pkg.LinterError {
	t.Helper()

	errorList := errors.NewLintRuleErrorsList()
	newRule(moduleAt(t, root), errorList).Check(context.Background())

	return errorList.GetErrors()
}

func layoutAt(t *testing.T, files, dirs []string) string {
	t.Helper()

	root := t.TempDir()

	for _, name := range files {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("x"), DefaultFilePerm))
	}

	for _, name := range dirs {
		require.NoError(t, os.Mkdir(filepath.Join(root, name), DefaultDirPerm))
	}

	return root
}
