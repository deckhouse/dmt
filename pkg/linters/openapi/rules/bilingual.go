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
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const docRuPrefix = "doc-ru-"

const BilingualRuleName = "bilingual"

type BilingualRule struct {
	pkg.RuleMeta
	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewBilingualRule(_ *pkg.OpenAPILinterConfig,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *BilingualRule {
	return &BilingualRule{
		RuleMeta: pkg.RuleMeta{
			Name: BilingualRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(BilingualRuleName),
	}
}

// Run checks that the given resource file has a corresponding doc-ru- translation file.
var _ pkg.Rule = (*BilingualRule)(nil)

func (r *BilingualRule) Check(_ context.Context) {
	for _, path := range bilingualCRDFiles(r.module.GetPath()) {
		r.checkFile(path)
	}
}

// checkFile must stay a separate method rather than being inlined into the
// loop above: it is the entry point the unit tests drive directly, with paths
// the walk in Check would never yield.
func (r *BilingualRule) checkFile(path string) {
	errorList := r.errorList

	shortPath := fsutils.Rel(r.module.GetPath(), path)
	filename := filepath.Base(path)
	dir := filepath.Dir(path)

	if strings.HasPrefix(filename, docRuPrefix) {
		// For doc-ru- files, check that the base file exists
		baseFilename := strings.TrimPrefix(filename, docRuPrefix)
		basePath := filepath.Join(dir, baseFilename)

		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			errorList.WithFilePath(shortPath).
				Errorf("translation file has no corresponding base file: expected %q", baseFilename)
		}

		return
	}

	// For base files, check that the doc-ru- counterpart exists
	docRuPath := filepath.Join(dir, docRuPrefix+filename)

	if _, err := os.Stat(docRuPath); os.IsNotExist(err) {
		errorList.WithFilePath(shortPath).
			Errorf("translation file is missing: expected %q in the same directory", docRuPrefix+filename)
	}
}
