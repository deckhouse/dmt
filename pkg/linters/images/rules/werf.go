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

	"k8s.io/utils/ptr"
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
	ImageSpec struct {
		Config struct {
			User string `json:"user" yaml:"user"`
		} `json:"config" yaml:"config"`
	} `json:"imageSpec" yaml:"imageSpec"`
}

type WerfRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

func NewWerfRule(disable bool) *WerfRule {
	return &WerfRule{
		RuleMeta: pkg.RuleMeta{
			Name: werfRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
	}
}

var excludeModules = []string{"terraform-manager"}

func isModuleExcluded(moduleName string) bool {
	for _, m := range excludeModules {
		if moduleName == m {
			return true
		}
	}
	return false
}

func (r *WerfRule) LintWerfFile(moduleName, data string, errorList *errors.LintRuleErrorsList) {
	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

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

		// Validate base image; exclude terraform-manager
		// terraform-manager uses its own base images
		err = isWerfImagesCorrect(w.FromImage)
		if err != nil {
			if isModuleExcluded(moduleName) {
				// Ignore errors for excluded modules
				errorList.WithMaxLevel(ptr.To(pkg.Ignored)).
					WithObjectID(fmt.Sprintf("werf.yaml:manifest-%d", i+1)).
					WithValue("fromImage: " + w.FromImage).
					Error(fmt.Sprintf("Invalid `fromImage:` value - %v", err))
			} else {
				errorList.WithObjectID(fmt.Sprintf("werf.yaml:manifest-%d", i+1)).
					WithValue("fromImage: " + w.FromImage).
					Error(fmt.Sprintf("Invalid `fromImage:` value - %v", err))
			}
		}

		// Validate imageSpec.config.user is not overridden
		if w.ImageSpec.Config.User != "" {
			// TODO: remove this check for istio and ingress-nginx modules
			if moduleName != "istio" && moduleName != "ingress-nginx" {
				errorList.WithObjectID(fmt.Sprintf("werf.yaml:manifest-%d", i+1)).
					WithValue("imageSpec.config.user: " + w.ImageSpec.Config.User).
					Error("`imageSpec.config.user:` parameter should be empty")
			} else {
				errorList.WithObjectID(fmt.Sprintf("werf.yaml:manifest-%d", i+1)).
					WithValue("imageSpec.config.user: " + w.ImageSpec.Config.User).
					Warn("`imageSpec.config.user:` parameter should be empty")
			}
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
func isWerfImagesCorrect(img string) error {
	if img == "" {
		return fmt.Errorf("image is empty")
	}

	// Split by '/' to analyze path components
	parts := strings.Split(img, "/")
	if len(parts) < 2 {
		return fmt.Errorf("image should be in format `base/<name>`, got %q", img)
	}

	// Check if the first component is "base" or "common"
	// TODO: remove "common" from this check
	switch parts[0] {
	case "base", "common":
		return nil
	default:
		return fmt.Errorf("image must start with `base/`, got %q", img)
	}
}
