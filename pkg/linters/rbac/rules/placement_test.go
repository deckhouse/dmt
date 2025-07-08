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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

func TestPlacementRule_Enabled(t *testing.T) {
	tests := []struct {
		name           string
		excludeRules   []pkg.KindRuleExclude
		kind           string
		objectName     string
		expectedResult bool
	}{
		{
			name:           "no exclusions - should be enabled",
			excludeRules:   []pkg.KindRuleExclude{},
			kind:           "ServiceAccount",
			objectName:     "test-sa",
			expectedResult: true,
		},
		{
			name: "excluded object - should be disabled",
			excludeRules: []pkg.KindRuleExclude{
				{Kind: "ServiceAccount", Name: "excluded-sa"},
			},
			kind:           "ServiceAccount",
			objectName:     "excluded-sa",
			expectedResult: false,
		},
		{
			name: "different kind - should be enabled",
			excludeRules: []pkg.KindRuleExclude{
				{Kind: "ServiceAccount", Name: "excluded-sa"},
			},
			kind:           "ClusterRole",
			objectName:     "excluded-sa",
			expectedResult: true,
		},
		{
			name: "different name - should be enabled",
			excludeRules: []pkg.KindRuleExclude{
				{Kind: "ServiceAccount", Name: "excluded-sa"},
			},
			kind:           "ServiceAccount",
			objectName:     "different-sa",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewPlacementRule(tt.excludeRules)
			result := rule.Enabled(tt.kind, tt.objectName)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPlacementRuleTracked_Enabled(t *testing.T) {
	tracker := exclusions.NewExclusionTracker()
	trackedRule := exclusions.NewTrackedKindRuleForModule(
		[]pkg.KindRuleExclude{
			{Kind: "ServiceAccount", Name: "tracked-sa"},
		},
		tracker,
		"rbac",
		"placement",
		"test-module",
	)

	rule := NewPlacementRuleTracked(trackedRule)

	// Test that excluded object is disabled
	assert.False(t, rule.Enabled("ServiceAccount", "tracked-sa"))

	// Test that non-excluded object is enabled
	assert.True(t, rule.Enabled("ServiceAccount", "other-sa"))

	// Test that different kind is enabled
	assert.True(t, rule.Enabled("ClusterRole", "tracked-sa"))
}

func TestPlacementRule_Constructor(t *testing.T) {
	excludeRules := []pkg.KindRuleExclude{
		{Kind: "ServiceAccount", Name: "test-sa"},
	}

	rule := NewPlacementRule(excludeRules)

	assert.Equal(t, "placement", rule.GetName())
	assert.Equal(t, excludeRules, rule.ExcludeRules)
}

func TestPlacementRuleTracked_Constructor(t *testing.T) {
	tracker := exclusions.NewExclusionTracker()
	trackedRule := exclusions.NewTrackedKindRuleForModule(
		[]pkg.KindRuleExclude{
			{Kind: "ServiceAccount", Name: "test-sa"},
		},
		tracker,
		"rbac",
		"placement",
		"test-module",
	)

	rule := NewPlacementRuleTracked(trackedRule)

	assert.Equal(t, "placement", rule.GetName())
	assert.Equal(t, trackedRule.ExcludeRules, rule.ExcludeRules)
	assert.Equal(t, trackedRule, rule.trackedRule)
}

func TestPlacementRule_ExclusionTracking(t *testing.T) {
	tracker := exclusions.NewExclusionTracker()
	trackedRule := exclusions.NewTrackedKindRuleForModule(
		[]pkg.KindRuleExclude{
			{Kind: "ServiceAccount", Name: "tracked-sa"},
			{Kind: "ClusterRole", Name: "tracked-role"},
		},
		tracker,
		"rbac",
		"placement",
		"test-module",
	)

	rule := NewPlacementRuleTracked(trackedRule)

	// Check that exclusions are registered
	unused := tracker.GetUnusedExclusions()
	assert.Contains(t, unused, "rbac")
	assert.Contains(t, unused["rbac"], "placement")
	assert.Contains(t, unused["rbac"]["placement"], "ServiceAccount/tracked-sa")
	assert.Contains(t, unused["rbac"]["placement"], "ClusterRole/tracked-role")

	// Use one exclusion
	rule.Enabled("ServiceAccount", "tracked-sa")

	// Check that used exclusion is marked
	unused = tracker.GetUnusedExclusions()
	assert.Contains(t, unused["rbac"]["placement"], "ClusterRole/tracked-role")
	assert.NotContains(t, unused["rbac"]["placement"], "ServiceAccount/tracked-sa")
}
