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

	"github.com/iancoleman/strcase"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container/rules"
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
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	manifests := fsutils.SplitManifests(m.GetWerfFile())
	checkGitSection(m.GetName(), manifests, errorList)
	checkUnderscoredImages(manifests, errorList)

	for _, object := range m.GetStorage() {
		checkTemplatesUsingRenderedImages(object, manifests, errorList)
	}
}

func checkGitSection(moduleName string, manifests []string, errorList *errors.LintRuleErrorsList) {
	for i, manifest := range manifests {
		jsonData, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			errorList.Errorf("parsing Werf file, document %d failed: %s", i+1, err)
			continue
		}

		imageName := gjson.GetBytes(jsonData, "image").String()
		if !strings.Contains(imageName, moduleName+"/") {
			continue
		}

		gjson.GetBytes(jsonData, "git").ForEach(func(_, value gjson.Result) bool {
			if !value.Get("stageDependencies").Exists() {
				errorList.Errorf("parsing Werf file, document %d (image: %s) failed: 'git.stageDependencies' is required", i+1, imageName)
				return false
			}

			return true
		})
	}
}

func checkTemplatesUsingRenderedImages(object storage.StoreObject, manifests []string, errorList *errors.LintRuleErrorsList) {
	images, err := rules.FindObjectRawImages(object.AbsPath)
	if err != nil {
		errorList.Errorf("finding object raw images failed: %s", err)
		return
	}

	for _, image := range images {
		kebabCaseImage := strcase.ToKebab(image)
		isContainerFound := false

		for _, manifest := range manifests {
			jsonData, err := yaml.YAMLToJSON([]byte(manifest))
			if err != nil {
				continue
			}

			imageName := gjson.GetBytes(jsonData, "image").String()

			if imageName == kebabCaseImage {
				isContainerFound = true
				break
			}
		}

		if !isContainerFound {
			errorList.Errorf("image %s is not found in the manifests", image)
		}
	}
}

func checkUnderscoredImages(manifests []string, errorList *errors.LintRuleErrorsList) {
	for i, manifest := range manifests {
		jsonData, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			errorList.Errorf("parsing Werf file, document %d failed: %s", i+1, err)
			continue
		}

		imageName := gjson.GetBytes(jsonData, "image").String()
		if imageName == "" {
			continue
		}

		if strings.Contains(imageName, "_") {
			errorList.Errorf("parsing Werf file, document %d (image: %s) failed: image name should not contain underscores", i+1, imageName)
		}
	}
}
