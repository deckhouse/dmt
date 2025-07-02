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
	"fmt"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	werfRuleName = "werf"
)

type werfFile struct {
	Artifact  string `json:"artifact" yaml:"artifact"`
	Image     string `json:"image" yaml:"image"`
	From      string `json:"from" yaml:"from"`
	Final     *bool  `json:"final" yaml:"final"`
	FromImage string `json:"fromImage" yaml:"fromImage"`
}

type WerfRule struct {
	pkg.RuleMeta
}

func NewWerfRule() *WerfRule {
	return &WerfRule{
		RuleMeta: pkg.RuleMeta{
			Name: werfRuleName,
		},
	}
}

func (r *WerfRule) LintWerfFile(moduleName, data string, errorList *errors.LintRuleErrorsList) {
	// Set rule name for all errors in this function
	errorList = errorList.WithRule(r.GetName())

	// Split YAML documents
	werfDocs := splitManifests(data)

	// Process each document
	for i, doc := range werfDocs {
		var w werfFile
		err := yaml.Unmarshal([]byte(doc), &w)
		if err != nil {
			// Log invalid YAML but continue processing other documents
			errorList.WithObjectID(fmt.Sprintf("werf.yaml:manifest-%d", i+1)).
				WithValue("yaml_error").
				Error(fmt.Sprintf("Invalid YAML document: %v", err))
			continue
		}

		// Skip if image is not in the module
		parts := strings.Split(w.Image, "/")
		if len(parts) < 2 {
			continue
		}

		if parts[0] != moduleName {
			continue
		}

		// Skip if no 'fromImage' field
		w.FromImage = strings.TrimSpace(w.FromImage)
		if w.FromImage == "" {
			continue
		}

		// Check for deprecated 'artifact' directive
		if w.Artifact != "" {
			errorList.WithObjectID(fmt.Sprintf("werf.yaml:manifest-%d", i+1)).
				WithValue("artifact: " + w.Artifact).
				Error("Use `from:` or `fromImage:` and `final: false` directives instead of `artifact:` in the werf file")
		}

		// Skip non-final images
		if w.Final != nil && !*w.Final {
			continue
		}

		// TODO: add skips for some images

		// Validate base image
		if !isWerfImagesCorrect(w.FromImage) {
			errorList.WithObjectID(fmt.Sprintf("werf.yaml:manifest-%d", i+1)).
				WithValue("fromImage: " + w.FromImage).
				Error("`fromImage:` parameter should be one of our `base` images")
		}
	}
}

// splitManifests splits YAML documents separated by '---' into a slice
func splitManifests(bigFile string) []string {
	var sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")

	// Trim whitespace to ensure proper document splitting
	bigFileTmp := strings.TrimSpace(bigFile)
	if bigFileTmp == "" {
		return []string{}
	}

	docs := sep.Split(bigFileTmp, -1)
	var result []string

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc != "" {
			result = append(result, doc)
		}
	}

	return result
}

// isWerfImagesCorrect validates that the image path contains `base_images`
func isWerfImagesCorrect(img string) bool {
	if img == "" {
		return false
	}

	// Split by '/' to analyze path components
	parts := strings.Split(img, "/")
	if len(parts) < 2 {
		return false
	}

	// Check if the first component is "base"
	return parts[0] == "base"
}
