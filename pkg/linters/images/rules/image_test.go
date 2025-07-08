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
	"path/filepath"
	"testing"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

func TestNewImageRuleTracked(t *testing.T) {
	tests := []struct {
		name           string
		excludeRules   []string
		expectedEnable bool
	}{
		{
			name:           "empty exclusions should enable rule",
			excludeRules:   []string{},
			expectedEnable: true,
		},
		{
			name:           "single exclusion should be registered",
			excludeRules:   []string{"test-image/"},
			expectedEnable: true,
		},
		{
			name:           "multiple exclusions should be registered",
			excludeRules:   []string{"test-image/", "another-image/"},
			expectedEnable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := exclusions.NewExclusionTracker()
			linterID := "images"
			ruleID := "image-file-path-prefix"
			moduleName := "test-module"

			// Convert string exclusions to PrefixRuleExclude
			excludeRules := make([]pkg.PrefixRuleExclude, len(tt.excludeRules))
			for i, rule := range tt.excludeRules {
				excludeRules[i] = pkg.PrefixRuleExclude(rule)
			}

			trackedRule := exclusions.NewTrackedPrefixRuleForModule(
				excludeRules,
				tracker,
				linterID,
				ruleID,
				moduleName,
			)

			rule := NewImageRuleTracked(trackedRule)

			// Verify rule was created correctly
			if rule.Name != dockerfileRuleName {
				t.Errorf("Expected rule name %s, got %s", dockerfileRuleName, rule.Name)
			}

			// Note: GetUsageStats only shows used exclusions, not registered ones
			// We'll verify registration by checking that the rule works correctly

			// Test Enabled behavior for a path that should not be excluded
			if !rule.Enabled("some-other-path") {
				t.Error("Expected rule to be enabled for non-excluded path")
			}
		})
	}
}

func TestImageRuleTracked_Enabled_Behavior(t *testing.T) {
	tests := []struct {
		name           string
		excludeRules   []string
		testPath       string
		expectedEnable bool
	}{
		{
			name:           "no exclusions - rule should be enabled",
			excludeRules:   []string{},
			testPath:       "any/path",
			expectedEnable: true,
		},
		{
			name:           "exclusion matches - rule should be disabled",
			excludeRules:   []string{"excluded/"},
			testPath:       "excluded/dockerfile",
			expectedEnable: false,
		},
		{
			name:           "exclusion doesn't match - rule should be enabled",
			excludeRules:   []string{"excluded/"},
			testPath:       "included/dockerfile",
			expectedEnable: true,
		},
		{
			name:           "multiple exclusions - first matches",
			excludeRules:   []string{"first/", "second/"},
			testPath:       "first/dockerfile",
			expectedEnable: false,
		},
		{
			name:           "multiple exclusions - second matches",
			excludeRules:   []string{"first/", "second/"},
			testPath:       "second/dockerfile",
			expectedEnable: false,
		},
		{
			name:           "multiple exclusions - none match",
			excludeRules:   []string{"first/", "second/"},
			testPath:       "third/dockerfile",
			expectedEnable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := exclusions.NewExclusionTracker()
			linterID := "images"
			ruleID := "image-file-path-prefix"
			moduleName := "test-module"

			// Convert string exclusions to PrefixRuleExclude
			excludeRules := make([]pkg.PrefixRuleExclude, len(tt.excludeRules))
			for i, rule := range tt.excludeRules {
				excludeRules[i] = pkg.PrefixRuleExclude(rule)
			}

			trackedRule := exclusions.NewTrackedPrefixRuleForModule(
				excludeRules,
				tracker,
				linterID,
				ruleID,
				moduleName,
			)

			rule := NewImageRuleTracked(trackedRule)

			// Test Enabled behavior
			result := rule.Enabled(tt.testPath)
			if result != tt.expectedEnable {
				t.Errorf("Expected Enabled() to return %v for path %s, got %v",
					tt.expectedEnable, tt.testPath, result)
			}
		})
	}
}

func TestImageRuleTracked_Integration_WithDockerfile(t *testing.T) {
	// Create temporary test directory structure
	tempDir := t.TempDir()
	imagesDir := filepath.Join(tempDir, ImagesDir)
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("Failed to create images directory: %v", err)
	}

	// Create a Dockerfile with unacceptable image in images directory
	dockerfilePath := filepath.Join(imagesDir, "Dockerfile")
	dockerfileContent := `FROM alpine:3.18
RUN echo "test"
`
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0600); err != nil {
		t.Fatalf("Failed to write test Dockerfile: %v", err)
	}

	tests := []struct {
		name           string
		excludeRules   []string
		expectedErrors int
	}{
		{
			name:           "no exclusions - should report error",
			excludeRules:   []string{},
			expectedErrors: 1,
		},
		{
			name:           "exclusion matches - should not report error",
			excludeRules:   []string{""}, // Empty string matches root of images directory
			expectedErrors: 0,
		},
		{
			name:           "exclusion doesn't match - should report error",
			excludeRules:   []string{"other/"},
			expectedErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := exclusions.NewExclusionTracker()
			linterID := "images"
			ruleID := "image-file-path-prefix"
			moduleName := "test-module"

			// Convert string exclusions to PrefixRuleExclude
			excludeRules := make([]pkg.PrefixRuleExclude, len(tt.excludeRules))
			for i, rule := range tt.excludeRules {
				excludeRules[i] = pkg.PrefixRuleExclude(rule)
			}

			trackedRule := exclusions.NewTrackedPrefixRuleForModule(
				excludeRules,
				tracker,
				linterID,
				ruleID,
				moduleName,
			)

			rule := NewImageRuleTracked(trackedRule)
			errorList := errors.NewLintRuleErrorsList()

			// Run the check
			rule.CheckImageNamesInDockerFiles(tempDir, errorList)

			// Verify error count
			errs := errorList.GetErrors()
			if len(errs) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectedErrors, len(errs))
			}
		})
	}
}

func TestImageRuleTracked_ExclusionTracking_Accuracy(t *testing.T) {
	tracker := exclusions.NewExclusionTracker()
	linterID := "images"
	ruleID := "image-file-path-prefix"
	moduleName := "test-module"

	excludeRules := []pkg.PrefixRuleExclude{
		"excluded1/",
		"excluded2/",
		"excluded3/",
	}

	trackedRule := exclusions.NewTrackedPrefixRuleForModule(
		excludeRules,
		tracker,
		linterID,
		ruleID,
		moduleName,
	)

	rule := NewImageRuleTracked(trackedRule)

	// Explicitly call Enabled for excluded1/, excluded2/, excluded3/
	for _, sub := range []string{"excluded1/", "excluded2/", "excluded3/"} {
		_ = rule.Enabled(sub + "Dockerfile")
	}
	_ = rule.Enabled("included/Dockerfile")

	usageStats := tracker.GetUsageStats()
	if len(usageStats) == 0 || len(usageStats[linterID]) == 0 || len(usageStats[linterID][ruleID]) == 0 {
		t.Fatal("Expected tracker to have usage statistics")
	}

	for _, excl := range []string{"excluded1/", "excluded2/", "excluded3/"} {
		if usageStats[linterID][ruleID][excl] == 0 {
			t.Errorf("Expected exclusion %s to be tracked as used", excl)
		}
	}

	if usageStats[linterID][ruleID]["included/"] != 0 {
		t.Errorf("Did not expect 'included/' to be tracked")
	}
}
