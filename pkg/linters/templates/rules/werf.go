/*
Copyright 2021 Flant JSC

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
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	WerfRuleName = "werf"
)

func NewWerfRule() *WerfRule {
	return &WerfRule{
		RuleMeta: pkg.RuleMeta{
			Name: WerfRuleName,
		},
	}
}

type WerfRule struct {
	pkg.RuleMeta
}

type iModule interface {
	GetWerfFile() string
	GetPath() string
}

func (r *WerfRule) ValidateWerfTemplates(m iModule, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	manifests := fsutils.SplitManifests(m.GetWerfFile())
	checkGitSection(manifests, errorList)
}

func checkGitSection(manifests []string, errorList *errors.LintRuleErrorsList) {
	for i, manifest := range manifests {
		jsonData, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			errorList.Errorf("parsing Werf file, document %d failed: %s", i+1, err)
			continue
		}
		gjson.GetBytes(jsonData, "git").ForEach(func(_, value gjson.Result) bool {
			if !value.Get("stageDependencies").Exists() {
				errorList.Errorf("parsing Werf file, document %d failed: 'git.stageDependencies' is required", i+1)
			}
			return true
		})
	}
}
