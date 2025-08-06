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

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	YamlRuleName = "yaml"
)

func NewYamlRule() *YamlRule {
	return &YamlRule{
		RuleMeta: pkg.RuleMeta{
			Name: YamlRuleName,
		},
	}
}

type YamlRule struct {
	pkg.RuleMeta
}

func (r *YamlRule) YamlModuleRule(moduleRoot string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	files := fsutils.GetFiles(moduleRoot, false, fsutils.FilterFileByExtensions(".yaml", ".yml"))
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			errorList.WithFilePath(file).Error(err.Error())
			continue
		}
		err = yaml.UnmarshalStrict(content, &map[string]any{})
		if err != nil {
			errorList.WithFilePath(file).Error(err.Error())
		}
	}
}
