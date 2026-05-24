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

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestCheckExprWithSourceLabel(t *testing.T) {
	tests := []struct {
		name           string
		expr           string
		recordNames    map[string]struct{}
		allowedMetrics map[string]struct{}
		expectedErrors int
	}{
		{
			name:           "metric with source=deckhouse has no errors",
			expr:           `kube_pod_info{source="deckhouse"}`,
			expectedErrors: 0,
		},
		{
			name:           "metric without source selector produces error",
			expr:           `kube_pod_info`,
			expectedErrors: 1,
		},
		{
			name:        	"recording rule name is allowed without source",
			expr:        	`my_recording_rule`,
			recordNames: 	map[string]struct{}{"my_recording_rule": {}},
			expectedErrors: 0,
		},
		{
			name:           "binary expr - one metric with source, one without",
			expr:           `a{source="deckhouse"} * on() b`,
			expectedErrors: 1,
		},
		{
			name:           "allowed metric does not produce error",
			expr:           `allowed_metric`,
			allowedMetrics: map[string]struct{}{"allowed_metric": {}},
			expectedErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recordNames := tt.recordNames
			if recordNames == nil {
				recordNames = make(map[string]struct{})
			}
			allowedMetrics := tt.allowedMetrics
			if allowedMetrics == nil {
				allowedMetrics = make(map[string]struct{})
			}

			rule := &SourceLabelRule{
				recordingRuleNames: recordNames,
				allowedMetrics:     allowedMetrics,
			}

			errorList := errors.NewLintRuleErrorsList()
			rule.checkExpr(tt.expr, "test-rule", "test-group", "/test/file.yaml", errorList)

			assert.Len(t, errorList.GetErrors(), tt.expectedErrors)
		})
	}
}

func TestSanitizeGrafanaExpr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replaces $__rate_interval with 5m",
			input:    `rate(m{source="deckhouse"}[$__rate_interval])`,
			expected: `rate(m{source="deckhouse"}[5m])`,
		},
		{
			name:     "replaces $namespace with __placeholder__",
			input:    `m{namespace="$namespace"}`,
			expected: `m{namespace="__placeholder__"}`,
		},
		{
			name:     "replaces ${var:regex} with __placeholder__",
			input:    `m{var="${var:regex}"}`,
			expected: `m{var="__placeholder__"}`,
		},
		{
			name:     "does not replace $source",
			input:    `m{source="$source"}`,
			expected: `m{source="$source"}`,
		},
		{
			name:     "replaces $__range with 5m",
			input:    `increase(m[$__range])`,
			expected: `increase(m[5m])`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeGrafanaExpr(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
