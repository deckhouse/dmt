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
