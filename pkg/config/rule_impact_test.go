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

package config

import (
	"testing"

	"github.com/deckhouse/dmt/pkg"
)

func TestRuleImpactConfiguration(t *testing.T) {
	// Test LintersSettings with rule-specific settings
	linterSettings := &LintersSettings{
		Container: ContainerSettings{
			Impact: &[]pkg.Level{pkg.Error}[0],
			RulesSettings: map[string]RuleSettings{
				"some-rule": {
					Impact: &[]pkg.Level{pkg.Warn}[0],
				},
				"another-rule": {
					Impact: &[]pkg.Level{pkg.Critical}[0],
				},
			},
		},
		Hooks: HooksSettings{
			Impact: &[]pkg.Level{pkg.Error}[0],
			RulesSettings: map[string]RuleSettings{
				"ingress-rule": {
					Impact: &[]pkg.Level{pkg.Warn}[0],
				},
			},
		},
		Images: ImageSettings{
			Impact: &[]pkg.Level{pkg.Error}[0],
		},
	}

	// Test rule-specific impact retrieval
	tests := []struct {
		linterName string
		ruleName   string
		expected   pkg.Level
	}{
		{"container", "some-rule", pkg.Warn},
		{"container", "another-rule", pkg.Critical},
		{"container", "non-existent-rule", pkg.Error}, // Should fall back to linter default
		{"hooks", "ingress-rule", pkg.Warn},
		{"hooks", "non-existent-rule", pkg.Error}, // Should fall back to linter default
		{"images", "any-rule", pkg.Error},         // Should fall back to linter default (no global config)
	}

	for _, test := range tests {
		t.Run(test.linterName+"-"+test.ruleName, func(t *testing.T) {
			impact := linterSettings.GetRuleImpact(test.linterName, test.ruleName)
			if impact == nil {
				t.Errorf("Expected impact for %s.%s, got nil", test.linterName, test.ruleName)
				return
			}
			if *impact != test.expected {
				t.Errorf("Expected impact %v for %s.%s, got %v", test.expected, test.linterName, test.ruleName, *impact)
			}
		})
	}
}

func TestRuleSettingsInitialization(t *testing.T) {
	linterSettings := &LintersSettings{}

	// Test that RulesSettings is properly initialized
	if linterSettings.Container.RulesSettings != nil {
		t.Error("Expected Container.RulesSettings to be nil initially")
	}

	// Test rule-specific impact retrieval with nil RulesSettings
	impact := linterSettings.GetRuleImpact("container", "some-rule")
	if impact != nil {
		t.Error("Expected nil impact for non-existent rule when RulesSettings is nil")
	}

	// Test with initialized RulesSettings
	linterSettings.Container.RulesSettings = map[string]RuleSettings{
		"some-rule": {
			Impact: &[]pkg.Level{pkg.Warn}[0],
		},
	}

	impact = linterSettings.GetRuleImpact("container", "some-rule")
	if impact == nil {
		t.Error("Expected impact for some-rule, got nil")
	} else if *impact != pkg.Warn {
		t.Errorf("Expected impact %v for some-rule, got %v", pkg.Warn, *impact)
	}
}

func TestRuleImpactFallback(t *testing.T) {
	linterSettings := &LintersSettings{
		Container: ContainerSettings{
			Impact: &[]pkg.Level{pkg.Warn}[0],
		},
	}

	// Test fallback to linter default when no global rule settings
	impact := linterSettings.GetRuleImpact("container", "any-rule")
	if impact == nil {
		t.Error("Expected impact for container rule, got nil")
		return
	}
	if *impact != pkg.Warn {
		t.Errorf("Expected impact %v for container rule, got %v", pkg.Warn, *impact)
	}
}
