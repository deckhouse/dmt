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

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// vendoredPatch is a third-party patch pulled in by yarn: neither its name nor
// its directory follows the module patch convention, so it is exactly what the
// exclude rules exist for.
const vendoredPatch = "images/app/ui/.yarn/patches/@ember-legacy-npm-0.4.2-51a29c3c1d.patch"

// patchesModuleFor builds a module tree with one conforming patch and one
// vendored patch, and returns a rule bound to it with the given exclusions.
func patchesModuleFor(t *testing.T, excludeFiles []pkg.StringRuleExclude,
	excludeDirs []pkg.DirectoryRuleExclude, errorList *errors.LintRuleErrorsList) *PatchesRule {
	t.Helper()

	moduleDir := t.TempDir()

	write := func(relPath, content string) {
		path := filepath.Join(moduleDir, relPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	write("images/app/patches/001-fix-ssl.patch", "diff")
	write("images/app/patches/README.md", "# 001-fix-ssl.patch\n")
	write(vendoredPatch, "diff")

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(moduleDir)

	return NewPatchesRule(false, excludeFiles, excludeDirs, m, errorList)
}

func TestPatchesRule_VendoredPatchReported(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	patchesModuleFor(t, nil, nil, errorList).Check(t.Context())

	found := errorList.GetErrors()

	texts := make([]string, 0, len(found))
	for _, e := range found {
		texts = append(texts, e.Text)
	}

	assert.True(t, errorList.ContainsErrors(), "Expected the vendored patch to be flagged")
	assert.Contains(t, texts, "Patch file name should match pattern `XXX-<patch-name>.patch`")
	assert.Contains(t, texts, "Patch file should have a corresponding README file")
}

func TestPatchesRule_ExcludeDirectory(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	excludeDirs := []pkg.DirectoryRuleExclude{"images/app/ui/.yarn"}
	patchesModuleFor(t, nil, excludeDirs, errorList).Check(t.Context())

	assert.Empty(t, errorList.GetErrors(), "Expected no findings for an excluded directory")
}

func TestPatchesRule_ExcludeFile(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	// Excluding the only patch file of a directory also silences the
	// directory-level checks: the directory is derived from the kept files.
	excludeFiles := []pkg.StringRuleExclude{pkg.StringRuleExclude(vendoredPatch)}
	patchesModuleFor(t, excludeFiles, nil, errorList).Check(t.Context())

	assert.Empty(t, errorList.GetErrors(), "Expected no findings for an excluded file")
}

func TestPatchesRule_ExcludeDoesNotHideOtherPatches(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	rule := patchesModuleFor(t, nil, []pkg.DirectoryRuleExclude{"images/app/ui/.yarn"}, errorList)

	// A patch outside the excluded directory is still checked.
	stray := filepath.Join(rule.module.GetPath(), "images/app/patches/no-number.patch")
	require.NoError(t, os.WriteFile(stray, []byte("diff"), 0o600))

	rule.Check(t.Context())

	assert.True(t, errorList.ContainsErrors(), "Expected the non-excluded patch to be flagged")

	for _, e := range errorList.GetErrors() {
		assert.NotContains(t, e.FilePath, ".yarn")
	}
}
