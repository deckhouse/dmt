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
	"bufio"
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

// Captures the last quoted argument of helm_lib_module_image on a line, supporting both:
//
//	image: {{ include "helm_lib_module_image" . "imageName" }}
//	image: {{ include "helm_lib_module_image" (list . "imageName") }}
var imageRawRegex = regexp.MustCompile(`image:.*helm_lib_module_image.*"([^"]+)"[^"]*$`)

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

	images, err := FindObjectRawImages(object.AbsPath)
	if err != nil {
		errorList.Errorf("Failed to read images from template file: %s", err)
		return
	}

	for _, image := range images {
		if strings.Contains(image, "_") {
			errorList.Errorf("Image name %q must not contain underscores", image)
		}
	}
}

// FindObjectRawImages finds all strings that match the imageRawRegex pattern in the given file.
// The returned strings are the last quoted arguments of helm_lib_module_image on a line, supporting both:
//
//	image: {{ include "helm_lib_module_image" . "imageName" }}
//	image: {{ include "helm_lib_module_image" (list . "imageName") }}
func FindObjectRawImages(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var images []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if match := imageRawRegex.FindStringSubmatch(scanner.Text()); len(match) >= 2 {
			images = append(images, match[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if images == nil {
		return []string{}, nil
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
