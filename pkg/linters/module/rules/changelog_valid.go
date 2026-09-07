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

// ChangelogValidRule reports a changelog.yaml the release image ships that cannot be parsed.
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

	var value any
	if err := yaml.Unmarshal(raw, &value); err != nil {
		errorList.Errorf("invalid YAML in %s:\n%s", ChangelogFilename, err)
	}
}
