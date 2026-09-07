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
	"bytes"
	"context"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/ignore"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const HelmignoreLeftoversRuleName = "helmignore-leftovers"

// LeftoversRule reports package-root entries the bundle image carries even though its
// own .helmignore excludes them.
//
// It is the bundle-scope half of what HelmignoreRule used to do in one pass, and the
// split is what the scopes are for. .helmignore takes effect when the chart is packed,
// so a source tree cannot say whether it did; a source tree under CI also holds scratch
// files the build never ships, and every one of those read as a finding. The image is
// what reaches a cluster, so it is the only tree where a leftover is a fact.
//
// Helm's own defaults are added to the parsed rules, so junk no .helmignore ever
// mentions — a .git directory shipped by accident — is reported too.
type LeftoversRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*LeftoversRule)(nil)

func NewHelmignoreLeftoversRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *LeftoversRule {
	return &LeftoversRule{
		RuleMeta:  pkg.RuleMeta{Name: HelmignoreLeftoversRuleName},
		module:    m,
		errorList: errorList.WithRule(HelmignoreLeftoversRuleName),
	}
}

func (r *LeftoversRule) Check(_ context.Context) {
	root := r.module.GetPath()
	if root == "" {
		return
	}

	raw, err := os.ReadFile(filepath.Join(root, helmignoreFile))
	if err != nil {
		// A missing .helmignore leaves nothing to compare the tree against. Its absence
		// is bundle-layout's finding to report, not a second copy of it here.
		if os.IsNotExist(err) {
			return
		}

		r.errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot read .helmignore file: %s", err)

		return
	}

	rules, err := ignore.Parse(bytes.NewReader(raw))
	if err != nil {
		r.errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot parse .helmignore: %s", err)

		return
	}

	rules.AddDefaults()

	entries, err := os.ReadDir(root)
	if err != nil {
		r.errorList.WithFilePath(helmignoreFile).
			Errorf("Cannot read package root: %s", err)

		return
	}

	for _, entry := range entries {
		r.checkEntry(rules, entry)
	}
}

// checkEntry reports one package-root entry that survived a pattern excluding it.
func (r *LeftoversRule) checkEntry(rules *ignore.Rules, entry os.DirEntry) {
	name := entry.Name()

	// .helmignore excludes itself by default, and the bundle ships it on purpose —
	// bundle-layout requires it to be there.
	if name == helmignoreFile {
		return
	}

	info, err := entry.Info()
	if err != nil {
		r.errorList.WithFilePath(name).
			Errorf("Cannot stat '%s': %s", name, err)

		return
	}

	if !rules.Ignore(name, info) {
		return
	}

	if shippedInBundle(name) {
		return
	}

	kind := "File"
	if entry.IsDir() {
		kind = "Directory"
		name += "/"
	}

	r.errorList.WithFilePath(entry.Name()).
		Errorf("%s '%s' is excluded by .helmignore but is present in the bundle image", kind, name)
}

// shippedInBundle reports whether a package-root entry that .helmignore excludes is
// nevertheless expected in the bundle image, and so is not a leftover.
//
// name is the bare entry name — no trailing slash on directories.
//
// TODO(human)
func shippedInBundle(name string) bool {
	return false
}
