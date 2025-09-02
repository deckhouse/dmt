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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"k8s.io/utils/ptr"
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
}

func NewPatchesRule(disable bool) *PatchesRule {
	return &PatchesRule{
		RuleMeta: pkg.RuleMeta{
			Name: PatchesRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
	}
}

func (r *PatchesRule) CheckPatches(moduleDir string, errorList *errors.LintRuleErrorsList) {
	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	errorList = errorList.WithRule(r.Name)

	files := fsutils.GetFiles(moduleDir, false, fsutils.FilterFileByExtensions(".patch"))
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
