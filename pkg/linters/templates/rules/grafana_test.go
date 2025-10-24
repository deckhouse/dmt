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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestExtractDashboardPanels(t *testing.T) {
	rule := &GrafanaRule{}

	// Test dashboard with direct panels
	dashboard := gjson.Parse(`{
		"panels": [
			{"type": "graph", "title": "Panel 1"},
			{"type": "row", "panels": [
				{"type": "timeseries", "title": "Panel 2"}
			]},
			{"type": "stat", "title": "Panel 3"}
		]
	}`)

	panels := rule.extractDashboardPanels(&dashboard)
	assert.Len(t, panels, 3)
	assert.Equal(t, "Panel 1", panels[0].Get("title").String())
	assert.Equal(t, "Panel 2", panels[1].Get("title").String())
	assert.Equal(t, "Panel 3", panels[2].Get("title").String())
}

func TestExtractDashboardTemplates(t *testing.T) {
	rule := &GrafanaRule{}

	// Test dashboard with templates
	dashboard := gjson.Parse(`{
		"templating": {
			"list": [
				{"name": "ds_prometheus", "type": "datasource", "query": "prometheus"},
				{"name": "namespace", "type": "query"}
			]
		}
	}`)

	templates := rule.extractDashboardTemplates(&dashboard)
	assert.Len(t, templates, 2)
	assert.Equal(t, "ds_prometheus", templates[0].Get("name").String())
	assert.Equal(t, "namespace", templates[1].Get("name").String())
}

func TestCheckDeprecatedPanelTypes(t *testing.T) {
	tests := []struct {
		name          string
		panelType     string
		expectedWarns int
	}{
		{
			name:          "deprecated graph panel",
			panelType:     "graph",
			expectedWarns: 1,
		},
		{
			name:          "deprecated flant-statusmap-panel",
			panelType:     "flant-statusmap-panel",
			expectedWarns: 1,
		},
		{
			name:          "modern timeseries panel",
			panelType:     "timeseries",
			expectedWarns: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &GrafanaRule{}
			errorList := errors.NewLintRuleErrorsList()

			panel := gjson.Parse(`{"type": "` + tt.panelType + `", "title": "Test Panel"}`)
			rule.checkDeprecatedPanelTypes(&panel, "Test Panel", "/test.json", errorList)

			assert.Len(t, errorList.GetErrors(), tt.expectedWarns)
		})
	}
}

func TestCheckDeprecatedIntervals(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		expectedWarns int
	}{
		{
			name:          "deprecated interval_rv",
			expr:          "rate(metric{job=\"test\"}[interval_rv])",
			expectedWarns: 1,
		},
		{
			name:          "deprecated interval_sx3",
			expr:          "rate(metric{job=\"test\"}[interval_sx3])",
			expectedWarns: 1,
		},
		{
			name:          "modern interval",
			expr:          "rate(metric{job=\"test\"}[$__rate_interval])",
			expectedWarns: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &GrafanaRule{}
			errorList := errors.NewLintRuleErrorsList()

			// Create JSON with proper escaping
			jsonStr := fmt.Sprintf(`{
				"title": "Test Panel",
				"targets": [{"expr": %q}]
			}`, tt.expr)

			panel := gjson.Parse(jsonStr)
			rule.checkDeprecatedIntervals(&panel, "Test Panel", "/test.json", errorList)

			assert.Len(t, errorList.GetErrors(), tt.expectedWarns)
		})
	}
}

func TestCheckLegacyAlertRules(t *testing.T) {
	tests := []struct {
		name          string
		hasAlert      bool
		alertName     string
		expectedWarns int
	}{
		{
			name:          "has legacy alert rule",
			hasAlert:      true,
			alertName:     "test_alert",
			expectedWarns: 1,
		},
		{
			name:          "no alert rule",
			hasAlert:      false,
			alertName:     "",
			expectedWarns: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &GrafanaRule{}
			errorList := errors.NewLintRuleErrorsList()

			var panel gjson.Result
			if tt.hasAlert {
				panel = gjson.Parse(`{
					"title": "Test Panel",
					"alert": {"name": "` + tt.alertName + `"}
				}`)
			} else {
				panel = gjson.Parse(`{"title": "Test Panel"}`)
			}

			rule.checkLegacyAlertRules(&panel, "Test Panel", "/test.json", errorList)

			assert.Len(t, errorList.GetErrors(), tt.expectedWarns)
		})
	}
}

func TestCheckDatasourceValidation(t *testing.T) {
	tests := []struct {
		name          string
		datasource    string
		expectedWarns int
	}{
		{
			name:          "recommended prometheus datasource",
			datasource:    `{"type": "prometheus", "uid": "$ds_prometheus"}`,
			expectedWarns: 0,
		},
		{
			name:          "hardcoded datasource UID",
			datasource:    `{"type": "prometheus", "uid": "prometheus-123"}`,
			expectedWarns: 2, // hardcoded UID + non-recommended prometheus UID
		},
		{
			name:          "legacy datasource format",
			datasource:    `"prometheus-123"`,
			expectedWarns: 2, // legacy format + hardcoded UID
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &GrafanaRule{}
			errorList := errors.NewLintRuleErrorsList()

			panel := gjson.Parse(`{
				"title": "Test Panel",
				"targets": [{"datasource": ` + tt.datasource + `}]
			}`)
			rule.checkDatasourceValidation(&panel, "Test Panel", "/test.json", errorList)

			assert.Len(t, errorList.GetErrors(), tt.expectedWarns)
		})
	}
}

func TestValidateTemplates(t *testing.T) {
	tests := []struct {
		name          string
		templates     string
		expectedWarns int
	}{
		{
			name:          "has required prometheus datasource variable",
			templates:     `[{"name": "ds_prometheus", "type": "datasource", "query": "prometheus"}, {"name": "namespace", "type": "query"}]`,
			expectedWarns: 0,
		},
		{
			name:          "missing required prometheus datasource variable",
			templates:     `[{"name": "namespace", "type": "query"}]`,
			expectedWarns: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &GrafanaRule{}
			errorList := errors.NewLintRuleErrorsList()

			templates := gjson.Parse(`{"templating": {"list": ` + tt.templates + `}}`)
			templateList := rule.extractDashboardTemplates(&templates)
			rule.validateTemplates(templateList, "/test.json", errorList)

			assert.Len(t, errorList.GetErrors(), tt.expectedWarns)
		})
	}
}

func TestValidateDashboardFile(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "grafana-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test dashboard file
	dashboardContent := `{
		"panels": [
			{"type": "graph", "title": "Deprecated Panel"}
		],
		"templating": {
			"list": [
				{"name": "ds_prometheus", "type": "datasource", "query": "prometheus"}
			]
		}
	}`

	testFile := filepath.Join(tempDir, "test-dashboard.json")
	err = os.WriteFile(testFile, []byte(dashboardContent), 0600)
	require.NoError(t, err)

	rule := &GrafanaRule{}

	errorList := errors.NewLintRuleErrorsList()
	rule.validateDashboardFile(testFile, errorList)

	// Should have warning about deprecated panel type
	errorListResult := errorList.GetErrors()
	assert.Len(t, errorListResult, 1)
	assert.Contains(t, errorListResult[0].Text, "deprecated type 'graph'")
}
