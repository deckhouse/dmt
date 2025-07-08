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

package promtool

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/promtool/rulefmt"
)

func TestCheckRules(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError bool
	}{
		{
			name:          "Valid rules",
			input:         `groups: [{name: "example", rules: [{alert: "HighRequestLatency", expr: "latency > 0.5"}]}]`,
			expectedError: false,
		},
		{
			name:          "Invalid rules",
			input:         `invalid yaml`,
			expectedError: true,
		},
		{
			name:          "Empty groups",
			input:         `groups: []`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRules([]byte(tt.input))
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckRuleGroups(t *testing.T) {
	tests := []struct {
		name          string
		ruleGroups    *rulefmt.RuleGroups
		expectedError bool
	}{
		{
			name: "Valid rule groups",
			ruleGroups: &rulefmt.RuleGroups{
				Groups: []rulefmt.RuleGroup{
					{
						Name: "example",
						Rules: []rulefmt.Rule{
							{Alert: "HighRequestLatency", Expr: "latency > 0.5"},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name:          "No rule groups",
			ruleGroups:    &rulefmt.RuleGroups{},
			expectedError: true,
		},
		{
			name:          "Nil rule groups",
			ruleGroups:    nil,
			expectedError: true,
		},
		{
			name: "Rule groups with duplicates",
			ruleGroups: &rulefmt.RuleGroups{
				Groups: []rulefmt.RuleGroup{
					{
						Name: "example",
						Rules: []rulefmt.Rule{
							{Alert: "HighRequestLatency", Expr: "latency > 0.5", Labels: map[string]string{"key": "value"}},
							{Alert: "HighRequestLatency", Expr: "latency > 0.6", Labels: map[string]string{"key": "value"}},
						},
					},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkRuleGroups(tt.ruleGroups, ls)
			if tt.expectedError {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCheckDuplicates(t *testing.T) {
	tests := []struct {
		name           string
		ruleGroups     []rulefmt.RuleGroup
		expectedResult []compareRuleType
	}{
		{
			name: "No duplicates",
			ruleGroups: []rulefmt.RuleGroup{
				{
					Name: "group1",
					Rules: []rulefmt.Rule{
						{Alert: "Alert1", Labels: map[string]string{"key": "value"}},
					},
				},
			},
			expectedResult: nil,
		},
		{
			name: "With duplicates",
			ruleGroups: []rulefmt.RuleGroup{
				{
					Name: "group1",
					Rules: []rulefmt.Rule{
						{Alert: "Alert1", Labels: map[string]string{"key": "value"}},
						{Alert: "Alert1", Labels: map[string]string{"key": "value"}},
					},
				},
			},
			expectedResult: []compareRuleType{
				{
					metric: "Alert1",
					label:  labels.FromMap(map[string]string{"key": "value"}),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkDuplicates(tt.ruleGroups)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestRuleMetric(t *testing.T) {
	tests := []struct {
		name     string
		rule     rulefmt.Rule
		expected string
	}{
		{
			name:     "Alert rule",
			rule:     rulefmt.Rule{Alert: "HighRequestLatency"},
			expected: "HighRequestLatency",
		},
		{
			name:     "Record rule",
			rule:     rulefmt.Rule{Record: "record_metric"},
			expected: "record_metric",
		},
		{
			name:     "Empty rule",
			rule:     rulefmt.Rule{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ruleMetric(tt.rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}
