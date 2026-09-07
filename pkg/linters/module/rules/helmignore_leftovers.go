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

	// bundle-layout requires .helmignore in the package root, so a pattern broad enough
	// to match it — `.*`, say — must not turn the file into a finding against itself.
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

// bundleShipped is the set of package-root entries a bundle image carries on purpose
// even though .helmignore excludes them.
var bundleShipped = map[string]bool{
	"Chart.yaml":          true,
	"images_digests.json": true,
	"module.yaml":         true,
	"charts":              true,
	"docs":                true,
	"openapi":             true,
	"templates":           true,

	"crds":       true,
	"hooks":      true,
	"monitoring": true,

	"changelog.yaml": true,
	"oss.yaml":       true,
	"package.yaml":   true,
}

// shippedInBundle reports whether a package-root entry that .helmignore excludes is
// nevertheless expected in the bundle image, and so is not a leftover.ё
func shippedInBundle(name string) bool {
	return bundleShipped[name]
}
