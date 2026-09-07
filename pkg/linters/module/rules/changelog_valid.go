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

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ChangelogValidRuleName = "changelog-valid"
	ChangelogFilename      = "changelog.yaml"
)

// ChangelogValidRule reports a changelog.yaml the release image ships that cannot be
// parsed. Deckhouse reads this file to render the release notes for a version, so a
// broken one is not a cosmetic problem: the release lands with no notes at all.
//
// Presence is not this rule's business — release-layout is what makes a missing
// changelog.yaml a finding, and this rule stays quiet when the file is absent. That is
// the same split definition-file and package-yaml follow, and it is what lets the rule
// be asked for by a scope whose image legitimately carries no changelog.
type ChangelogValidRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*ChangelogValidRule)(nil)

func NewChangelogValidRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *ChangelogValidRule {
	return &ChangelogValidRule{
		RuleMeta:  pkg.RuleMeta{Name: ChangelogValidRuleName},
		module:    m,
		errorList: errorList.WithRule(ChangelogValidRuleName),
	}
}

func (r *ChangelogValidRule) Check(_ context.Context) {
	root := r.module.GetPath()
	if root == "" {
		return
	}

	errorList := r.errorList.WithFilePath(ChangelogFilename)

	raw, err := os.ReadFile(filepath.Join(root, ChangelogFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return
		}

		errorList.Errorf("Cannot read %s: %s", ChangelogFilename, err)

		return
	}

	// Syntax only, on purpose: the shape of a changelog entry is the changelog
	// builder's business and grows without warning, so asserting one here would turn
	// the next format addition into a finding against a release that is perfectly
	// fine. An empty file parses to nil and is therefore not a finding either — the
	// changelog rule is what reports an empty changelog, in the source tree.
	var value any
	if err := yaml.Unmarshal(raw, &value); err != nil {
		errorList.Errorf("invalid YAML in %s:\n%s", ChangelogFilename, err)
	}
}
