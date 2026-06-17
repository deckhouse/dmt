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
	"strings"

	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/werf"
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

func (r *WerfRule) ValidateWerfTemplates(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	imageFiles, err := werf.GetModuleImagesWerfFiles(m.GetPath())
	if err != nil {
		errorList.Errorf("Failed to read images werf files: %s", err)
		return
	}

	for _, imageFile := range imageFiles {
		fileErrorList := errorList.WithFilePath(imageFile.RelPath)

		manifests := fsutils.SplitManifests(imageFile.Content)
		checkUnderscoredImages(imageFile.RelPath, manifests, fileErrorList)
	}
}

func checkUnderscoredImages(filePath string, manifests []string, errorList *errors.LintRuleErrorsList) {
	for i, manifest := range manifests {
		jsonData, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			errorList.Errorf("Failed to parse %s document %d: %s", filePath, i+1, err)
			continue
		}

		imageName := gjson.GetBytes(jsonData, "image").String()
		if imageName == "" {
			continue
		}

		if strings.Contains(imageName, "_") {
			errorList.Errorf("Image name %q in %s (document %d) must not contain underscores", imageName, filePath, i+1)
		}
	}
}
