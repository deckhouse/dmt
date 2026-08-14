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
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// newModuleWithOversizedDoc creates a module whose docs/big.md is just over the
// size limit, and returns the module mock.
func newModuleWithOversizedDoc(t *testing.T) pkg.Module {
	t.Helper()

	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)

	tempDir := t.TempDir()
	mockModule.GetPathMock.Return(tempDir)

	docFile := filepath.Join(tempDir, "docs", "big.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(docFile), 0o755))

	f, err := os.Create(docFile)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(fsutils.MaxLintableFileSize+1))
	require.NoError(t, f.Close())

	return mockModule
}

func TestCyrillicInEnglish_OversizedFile_WarnsAndCanBeExcluded(t *testing.T) {
	t.Run("warns when not excluded", func(t *testing.T) {
		m := newModuleWithOversizedDoc(t)
		errorList := errors.NewLintRuleErrorsList()

		NewCyrillicInEnglishRule().CheckFiles(m, errorList)

		errs := errorList.GetErrors()
		require.Len(t, errs, 1)
		require.True(t, strings.EqualFold(errs[0].Level.String(), "warn"), "level=%s", errs[0].Level.String())
		require.Contains(t, errs[0].Text, "too large")
	})

	t.Run("excluded by file", func(t *testing.T) {
		m := newModuleWithOversizedDoc(t)
		errorList := errors.NewLintRuleErrorsList()

		NewCyrillicInEnglishRule().
			WithFileSizeExcludes([]pkg.StringRuleExclude{"docs/big.md"}, nil).
			CheckFiles(m, errorList)

		require.Empty(t, errorList.GetErrors())
	})

	t.Run("excluded by directory", func(t *testing.T) {
		m := newModuleWithOversizedDoc(t)
		errorList := errors.NewLintRuleErrorsList()

		NewCyrillicInEnglishRule().
			WithFileSizeExcludes(nil, []pkg.DirectoryRuleExclude{"docs"}).
			CheckFiles(m, errorList)

		require.Empty(t, errorList.GetErrors())
	})
}

func TestNoLangKey_OversizedFile_WarnsAndCanBeExcluded(t *testing.T) {
	t.Run("warns when not excluded", func(t *testing.T) {
		m := newModuleWithOversizedDoc(t)
		errorList := errors.NewLintRuleErrorsList()

		NewNoLangKeyRule().CheckFiles(m, errorList)

		errs := errorList.GetErrors()
		require.Len(t, errs, 1)
		require.True(t, strings.EqualFold(errs[0].Level.String(), "warn"), "level=%s", errs[0].Level.String())
		require.Contains(t, errs[0].Text, "too large")
	})

	t.Run("excluded by file", func(t *testing.T) {
		m := newModuleWithOversizedDoc(t)
		errorList := errors.NewLintRuleErrorsList()

		NewNoLangKeyRule().
			WithFileSizeExcludes([]pkg.StringRuleExclude{"docs/big.md"}, nil).
			CheckFiles(m, errorList)

		require.Empty(t, errorList.GetErrors())
	})

	t.Run("excluded by directory", func(t *testing.T) {
		m := newModuleWithOversizedDoc(t)
		errorList := errors.NewLintRuleErrorsList()

		NewNoLangKeyRule().
			WithFileSizeExcludes(nil, []pkg.DirectoryRuleExclude{"docs"}).
			CheckFiles(m, errorList)

		require.Empty(t, errorList.GetErrors())
	})
}
