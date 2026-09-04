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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PatchesRuleName = "patches"
)

var (
	regexPatchFile = regexp.MustCompile(`^\d{3}-.*\.patch$`)
	regexPatchDir  = regexp.MustCompile(`^images/[\w/\-.]*/patches.*$`)
)

type PatchesRule struct {
	pkg.RuleMeta
	pkg.BoolRule

	// paths skips individual patch files and whole directories, matched by their
	// module-relative path. It is a named field rather than an embedded
	// pkg.PathRule because BoolRule already provides Enabled().
	paths pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewPatchesRule(disable bool,
	excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *PatchesRule {
	return &PatchesRule{
		RuleMeta: pkg.RuleMeta{
			Name: PatchesRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
		paths: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
		},
		module:    m,
		errorList: errorList.WithRule(PatchesRuleName),
	}
}

var _ pkg.Rule = (*PatchesRule)(nil)

func (r *PatchesRule) Check(_ context.Context) {
	moduleDir := r.module.GetPath()

	errorList := r.errorList
	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	// Excluded files are dropped before the patch directories are derived from
	// them, so excluding a directory (or every patch file inside it) also silences
	// the directory-level checks below.
	files := make([]string, 0)

	for _, file := range fsutils.GetFiles(moduleDir, false, fsutils.FilterFileByExtensions(".patch")) {
		if !r.paths.Enabled(fsutils.Rel(moduleDir, file)) {
			continue
		}

		files = append(files, file)
	}

	patchDirs := set.New()
	for _, file := range files {
		patchDirs.Add(filepath.Dir(file))
	}

	for _, patchDir := range patchDirs.Slice() {
		path := fsutils.Rel(moduleDir, patchDir)
		if !regexPatchDir.MatchString(path) {
			errorList.WithFilePath(path).Errorf("Patch file should be in `images/<image_name>/patches/` directory")
		}

		if !fsutils.IsFile(filepath.Join(patchDir, "README.md")) {
			errorList.WithFilePath(path).Errorf("Patch file should have a corresponding README file")
		}
	}

	for file := range slices.Values(files) {
		path := fsutils.Rel(moduleDir, file)
		if !regexPatchFile.MatchString(filepath.Base(file)) {
			errorList.WithFilePath(path).Errorf("Patch file name should match pattern `XXX-<patch-name>.patch`")
		}

		if err := checkReadmeFile(file); err != nil {
			errorList.WithFilePath(path).Errorf("%s", err.Error())
		}
	}
}

func checkReadmeFile(patchFile string) error {
	readmeFile := filepath.Join(filepath.Dir(patchFile), "README.md")

	content, err := os.ReadFile(readmeFile)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error reading README.md file: %w", err)
	}

	if !strings.Contains(string(content), "# "+filepath.Base(patchFile)) {
		return fmt.Errorf("%s", "README.md file does not contain # "+filepath.Base(patchFile))
	}

	return nil
}
