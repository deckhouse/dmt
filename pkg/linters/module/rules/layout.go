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

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ReleaseLayoutRuleName = "release-layout"
	BundleLayoutRuleName  = "bundle-layout"
)

// LayoutRule reports the files and directories a built image is missing from its
// package root.
//
// It exists as its own rule rather than as a flag on the rules that parse those
// files because presence is a property of the scope, not of the file: a source
// tree may legitimately lack version.json, an image may not. The parsing rules
// therefore keep returning quietly when a file is absent, and this rule is what
// the release and bundle scopes ask for to make the absence a finding.
type LayoutRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
	files     []string
	dirs      []string
}

var _ pkg.Rule = (*LayoutRule)(nil)

// NewReleaseLayoutRule checks the root of a release image, which carries only the
// metadata Deckhouse reads to decide whether to install the version.
func NewReleaseLayoutRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *LayoutRule {
	return newLayoutRule(ReleaseLayoutRuleName, m, errorList,
		[]string{"module.yaml", "version.json", "changelog.yaml"},
		nil,
	)
}

// NewBundleLayoutRule checks the root of a bundle image, which carries the whole
// packaged module — chart, templates and docs included.
//
// The list is what a published bundle actually holds, which is not what its source
// tree holds: changelog.yaml and version.json ship in the sibling release image, and
// the ignore file the package carries is .helmignore, not .gitignore.
//
// It is the intersection of eight published CE bundles, not every path they carry —
// crds/, hooks/, monitoring/ and .werf/ appear in some and not others, so requiring
// any of them would fail the modules that legitimately have nothing to put there.
func NewBundleLayoutRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *LayoutRule {
	return newLayoutRule(BundleLayoutRuleName, m, errorList,
		[]string{".helmignore", "Chart.yaml", "images_digests.json", "module.yaml"},
		[]string{"charts", "docs", "openapi", "templates"},
	)
}

func newLayoutRule(name string, m pkg.Module, errorList *errors.LintRuleErrorsList, files, dirs []string) *LayoutRule {
	return &LayoutRule{
		RuleMeta:  pkg.RuleMeta{Name: name},
		module:    m,
		errorList: errorList.WithRule(name),
		files:     files,
		dirs:      dirs,
	}
}

func (r *LayoutRule) Check(_ context.Context) {
	root := r.module.GetPath()
	if root == "" {
		return
	}

	for _, name := range r.files {
		r.check(root, name, false)
	}

	for _, name := range r.dirs {
		r.check(root, name, true)
	}
}

// check reports the three outcomes apart: the path is absent, it exists but is of
// the wrong kind, or it could not be read at all.
func (r *LayoutRule) check(root, name string, wantDir bool) {
	kind := "file"
	if wantDir {
		kind = "directory"
	}

	path := filepath.Join(root, name)
	errorList := r.errorList.WithFilePath(name)

	info, err := os.Stat(path)

	switch {
	case os.IsNotExist(err):
		errorList.Errorf("%s %s is missing in package root", name, kind)
	case err != nil:
		errorList.WithValue(err.Error()).Errorf("failed to check %s %s", name, kind)
	case info.IsDir() != wantDir:
		errorList.Errorf("%s must be a %s in package root", name, kind)
	}
}
