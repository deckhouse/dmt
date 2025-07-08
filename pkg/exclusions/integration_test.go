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

	"github.com/deckhouse/dmt/pkg"
)

func TestIntegrationWithLinters(t *testing.T) {
	// Create a tracker
	tracker := NewExclusionTracker()

	// Simulate linter usage
	// Container linter - DNS Policy rule
	dnsPolicyRule := NewTrackedKindRule(
		[]pkg.KindRuleExclude{
			{Kind: "Deployment", Name: "test-deployment"},
			{Kind: "DaemonSet", Name: "unused-daemonset"},
		},
		tracker,
		"container",
		"dns-policy",
	)

	// Mark one exclusion as used
	dnsPolicyRule.Enabled("Deployment", "test-deployment")

	// Container linter - Security Context rule
	securityContextRule := NewTrackedKindRule(
		[]pkg.KindRuleExclude{
			{Kind: "Deployment", Name: "secure-deployment"},
		},
		tracker,
		"container",
		"security-context",
	)

	// Mark this exclusion as used
	securityContextRule.Enabled("Deployment", "secure-deployment")

	// NoCyrillic linter - Files rule
	filesRule := NewTrackedPathRule(
		[]pkg.StringRuleExclude{
			"docs/README_ru.md",
			"docs/unused_file_ru.md",
		},
		[]pkg.PrefixRuleExclude{
			"docs/ru/",
			"legacy/docs/ru/",
		},
		tracker,
		"no-cyrillic",
		"files",
	)

	// Mark some exclusions as used
	filesRule.Enabled("docs/README_ru.md")
	filesRule.Enabled("docs/ru/")

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// Should have unused exclusions
	assert.Len(t, unused, 2) // container and no-cyrillic

	// Check container unused exclusions
	assert.Contains(t, unused, "container")
	assert.Len(t, unused["container"], 1)
	assert.Contains(t, unused["container"], "dns-policy")
	assert.Equal(t, []string{"DaemonSet/unused-daemonset"}, unused["container"]["dns-policy"])

	// Check no-cyrillic unused exclusions
	assert.Contains(t, unused, "no-cyrillic")
	assert.Len(t, unused["no-cyrillic"], 1)
	assert.Contains(t, unused["no-cyrillic"], "files")
	assert.Contains(t, unused["no-cyrillic"]["files"], "docs/unused_file_ru.md")
	assert.Contains(t, unused["no-cyrillic"]["files"], "legacy/docs/ru/")

	// Test formatting
	formatted := tracker.FormatUnusedExclusions()
	assert.Contains(t, formatted, "container:")
	assert.Contains(t, formatted, "no-cyrillic:")
	assert.Contains(t, formatted, "DaemonSet/unused-daemonset")
	assert.Contains(t, formatted, "docs/unused_file_ru.md")
	assert.Contains(t, formatted, "legacy/docs/ru/")
}

func TestIntegrationWithBoolRules(t *testing.T) {
	tracker := NewExclusionTracker()

	// Test bool rule (like disabled rules)
	boolRule := NewTrackedBoolRule(
		true, // disabled
		tracker,
		"templates",
		"grafana-dashboards",
	)

	// Check if rule is enabled (should be false since it's disabled)
	enabled := boolRule.Enabled()
	assert.False(t, enabled)

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// When a bool rule is disabled, it should be marked as used and not appear in unused list
	// So the unused list should be empty
	assert.Len(t, unused, 0)

	// Test with enabled rule
	tracker2 := NewExclusionTracker()
	boolRule2 := NewTrackedBoolRule(
		false, // enabled
		tracker2,
		"templates",
		"grafana-dashboards",
	)

	// Check if rule is enabled (should be true since it's enabled)
	enabled2 := boolRule2.Enabled()
	assert.True(t, enabled2)

	// Get unused exclusions
	unused2 := tracker2.GetUnusedExclusions()

	// When a bool rule is enabled, there are no exclusions to track
	assert.Len(t, unused2, 0)
}

func TestIntegrationWithStringRules(t *testing.T) {
	tracker := NewExclusionTracker()

	// Test string rule
	stringRule := NewTrackedStringRule(
		[]pkg.StringRuleExclude{
			"active-service-account",
			"unused-service-account",
		},
		tracker,
		"rbac",
		"binding-subject",
	)

	// Mark one exclusion as used
	stringRule.Enabled("active-service-account")

	// Get unused exclusions
	unused := tracker.GetUnusedExclusions()

	// Should have one unused exclusion
	assert.Len(t, unused, 1)
	assert.Contains(t, unused, "rbac")
	assert.Len(t, unused["rbac"], 1)
	assert.Contains(t, unused["rbac"], "binding-subject")
	assert.Equal(t, []string{"unused-service-account"}, unused["rbac"]["binding-subject"])
}
