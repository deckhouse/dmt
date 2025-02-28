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
	"slices"
	"strings"

	"regexp"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PatchesRuleName = "patches"
)

var (
	regexPatchFile = regexp.MustCompile(`^\d{3}-.*\.patch$`)
	regexPatchDir  = regexp.MustCompile(`^images/{a-z,A-Z}*/patches/.*$`)
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
	errorList = errorList.WithRule(r.Name)

	if !r.Enabled() {
		return
	}

	for file := range slices.Values(fsutils.GetFiles(moduleDir, false, filterPatches)) {
		errorList = errorList.WithFilePath(fsutils.Rel(moduleDir, file))
		if !regexPatchFile.MatchString(filepath.Base(file)) {
			errorList.Errorf("Patch file name should match pattern `XXX-<description>.patch`")
		}
		if !regexPatchDir.MatchString(fsutils.Rel(moduleDir, file)) {
			errorList.Errorf("Patch file should be in `images/<image_name>/patches/` directory")
		}
		if !checkReadmeFileExist(file) {
			errorList.Errorf("Patch file should have a corresponding README file")
			continue
		}
		if !checkReadmeFile(file) {
			errorList.Errorf("README file should contain a description of the patch")
		}
	}
}

// filterPatches will get all patch files
func filterPatches(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		logger.DebugF("Error getting file info: %v", err)
		return false
	}
	if f.IsDir() {
		return false
	}
	if filepath.Ext(path) == ".patch" {
		return true
	}

	return false
}

func checkReadmeFileExist(patchFile string) bool {
	readmeFile := filepath.Join(filepath.Dir(patchFile), "README.md")
	if _, err := os.Stat(readmeFile); err != nil {
		return false
	}
	return true
}

func checkReadmeFile(patchFile string) bool {
	readmeFile := filepath.Join(filepath.Dir(patchFile), "README.md")
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		return false
	}
	if len(content) == 0 {
		return false
	}
	if !strings.Contains(string(content), "# "+filepath.Base(patchFile)) {
		return false
	}

	return true
}
