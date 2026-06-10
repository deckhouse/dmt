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
	"io"
	"os"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ImageNoUnderscoresRuleName = "image-no-underscores"
)

var imageRawRegex = regexp.MustCompile(`.*image:.*"(.*)".*`)

func NewImageNoUnderscoresRule(excludeRules []pkg.ContainerRuleExclude) *ImageNoUnderscoresRule {
	return &ImageNoUnderscoresRule{
		RuleMeta: pkg.RuleMeta{
			Name: ImageNoUnderscoresRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type ImageNoUnderscoresRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

func (r *ImageNoUnderscoresRule) ContainerImageNoUnderscoresCheck(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	file, err := os.Open(object.AbsPath)
	if err != nil {
		errorList.Errorf("opening file %s failed: %s", object.GetPath(), err)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		errorList.Errorf("reading file %s failed: %s", object.GetPath(), err)
		return
	}

	images, err := FindContainerRawImages(string(content))
	if err != nil {
		errorList.Errorf("finding container raw images failed: %s", err)
		return
	}

	for _, image := range images {
		if strings.Contains(image, "_") {
			errorList.Errorf("image %s contains underscores", image)
		}
	}

}

func FindContainerRawImages(content string) ([]string, error) {
	matches := imageRawRegex.FindAllStringSubmatch(content, -1)

	images := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		images = append(images, match[1])
	}

	return images, nil
}

func (r *ImageNoUnderscoresRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(object, container) {
			return false
		}
	}

	return true
}
