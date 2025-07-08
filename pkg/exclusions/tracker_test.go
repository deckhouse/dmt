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

package exclusions

import (
	"testing"

	"github.com/deckhouse/dmt/pkg"
	"github.com/stretchr/testify/assert"
)

func TestExclusionTracker(t *testing.T) {
	tracker := NewExclusionTracker()

	// Register exclusions for a linter and rule
	tracker.RegisterExclusions("container", "dns-policy", []string{
		"Deployment/test-deployment",
		"DaemonSet/test-daemonset",
		"StatefulSet/unused-statefulset",
	})

	// Mark some exclusions as used
	tracker.MarkExclusionUsed("container", "dns-policy", "Deployment/test-deployment")
	tracker.MarkExclusionUsed("container", "dns-policy", "DaemonSet/test-daemonset")

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// Should have one unused exclusion
	assert.Len(t, unused, 1)
	assert.Contains(t, unused, "container")
	assert.Len(t, unused["container"], 1)
	assert.Contains(t, unused["container"], "dns-policy")
	assert.Equal(t, []string{"StatefulSet/unused-statefulset"}, unused["container"]["dns-policy"])

	// Test formatting
	formatted := tracker.FormatUnusedExclusions()
	expected := "Unused exclusions found:\n  container:\n    dns-policy:\n      - StatefulSet/unused-statefulset\n"
	assert.Equal(t, expected, formatted)
}

func TestExclusionTrackerMultipleRules(t *testing.T) {
	tracker := NewExclusionTracker()

	// Register exclusions for multiple rules
	tracker.RegisterExclusions("container", "dns-policy", []string{
		"Deployment/test-deployment",
		"DaemonSet/test-daemonset",
	})
	tracker.RegisterExclusions("container", "security-context", []string{
		"Deployment/secure-deployment",
	})

	// Mark some exclusions as used
	tracker.MarkExclusionUsed("container", "dns-policy", "Deployment/test-deployment")
	tracker.MarkExclusionUsed("container", "security-context", "Deployment/secure-deployment")

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// Should have one unused exclusion in dns-policy rule
	assert.Len(t, unused, 1)
	assert.Contains(t, unused, "container")
	assert.Len(t, unused["container"], 1)
	assert.Contains(t, unused["container"], "dns-policy")
	assert.Equal(t, []string{"DaemonSet/test-daemonset"}, unused["container"]["dns-policy"])
}

func TestExclusionTrackerNoUnused(t *testing.T) {
	tracker := NewExclusionTracker()

	// Register exclusions
	tracker.RegisterExclusions("container", "dns-policy", []string{
		"Deployment/test-deployment",
	})

	// Mark all exclusions as used
	tracker.MarkExclusionUsed("container", "dns-policy", "Deployment/test-deployment")

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// Should have no unused exclusions
	assert.Len(t, unused, 0)

	// Test formatting
	formatted := tracker.FormatUnusedExclusions()
	assert.Equal(t, "", formatted)
}

func TestExclusionTrackerUsageStats(t *testing.T) {
	tracker := NewExclusionTracker()

	// Register exclusions
	tracker.RegisterExclusions("container", "dns-policy", []string{
		"Deployment/test-deployment",
		"DaemonSet/test-daemonset",
	})

	// Mark exclusions as used multiple times
	tracker.MarkExclusionUsed("container", "dns-policy", "Deployment/test-deployment")
	tracker.MarkExclusionUsed("container", "dns-policy", "Deployment/test-deployment")
	tracker.MarkExclusionUsed("container", "dns-policy", "DaemonSet/test-daemonset")

	// Get usage stats
	stats := tracker.GetUsageStats()

	// Check usage counts
	assert.Len(t, stats, 1)
	assert.Contains(t, stats, "container")
	assert.Len(t, stats["container"], 1)
	assert.Contains(t, stats["container"], "dns-policy")
	assert.Equal(t, 2, stats["container"]["dns-policy"]["Deployment/test-deployment"])
	assert.Equal(t, 1, stats["container"]["dns-policy"]["DaemonSet/test-daemonset"])
}

func TestExclusionTrackerWithModules(t *testing.T) {
	tracker := NewExclusionTracker()

	// Register exclusions for different modules
	tracker.RegisterExclusionsForModule("container", "dns-policy", []string{"Deployment/test-deployment"}, "module1")
	tracker.RegisterExclusionsForModule("container", "dns-policy", []string{"DaemonSet/test-daemonset"}, "module2")

	// Mark one exclusion as used
	tracker.MarkExclusionUsed("container", "dns-policy", "Deployment/test-deployment")

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// Should have one unused exclusion from module2
	assert.Len(t, unused, 1)
	assert.Contains(t, unused, "container")
	assert.Len(t, unused["container"], 1)
	assert.Contains(t, unused["container"], "dns-policy")
	assert.Equal(t, []string{"DaemonSet/test-daemonset"}, unused["container"]["dns-policy"])

	// Check that the unused exclusion shows module information
	unusedFormatted := tracker.FormatUnusedExclusions()
	assert.Contains(t, unusedFormatted, "(from modules: module2)")
}

func TestExclusionTrackerWithUnprocessedFiles(t *testing.T) {
	tracker := NewExclusionTracker()

	// Register exclusions for license rule
	exclusions := []string{
		"images/simple-bridge/src/rootfs/bin/simple-bridge",
		"some/other/file.go",
	}
	tracker.RegisterExclusionsForModule("module", "license", exclusions, "test-module")

	// Simulate that only one file is processed by the linter
	// (the .go file would be processed, but the binary file would not)
	tracker.MarkExclusionUsed("module", "license", "some/other/file.go")

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// The binary file should be marked as unused because it was never processed
	if len(unused["module"]["license"]) != 1 {
		t.Errorf("Expected 1 unused exclusion, got %d", len(unused["module"]["license"]))
	}

	if unused["module"]["license"][0] != "images/simple-bridge/src/rootfs/bin/simple-bridge" {
		t.Errorf("Expected unused exclusion to be 'images/simple-bridge/src/rootfs/bin/simple-bridge', got '%s'", unused["module"]["license"][0])
	}
}

func TestExclusionTrackerWithTemplatesLinter(t *testing.T) {
	tracker := NewExclusionTracker()
	
	// Register VPA exclusions for templates linter
	exclusions := []pkg.KindRuleExclude{
		{
			Kind: "Deployment",
			Name: "standby-holder-name",
		},
		{
			Kind: "Deployment", 
			Name: "non-existent-deployment",
		},
	}
	
	// Register exclusions in tracker
	tracker.RegisterExclusionsForModule("templates", "vpa", []string{}, "test-module")
	
	// Create tracked rule with exclusions
	trackedRule := NewTrackedKindRuleForModule(
		exclusions,
		tracker,
		"templates",
		"vpa",
		"test-module",
	)
	
	// Simulate processing of objects
	// Only the first deployment exists and is processed
	trackedRule.Enabled("Deployment", "standby-holder-name") // This should mark the exclusion as used
	
	// The second deployment doesn't exist, so its exclusion should remain unused
	
	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()
	
	// The second exclusion should be marked as unused because it was never applied to a real object
	if len(unused["templates"]["vpa"]) != 1 {
		t.Errorf("Expected 1 unused exclusion, got %d", len(unused["templates"]["vpa"]))
	}
	
	expectedUnused := "kind = Deployment ; name = non-existent-deployment"
	if unused["templates"]["vpa"][0] != expectedUnused {
		t.Errorf("Expected unused exclusion '%s', got '%s'", expectedUnused, unused["templates"]["vpa"][0])
	}
}
