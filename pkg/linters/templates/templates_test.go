package templates

import (
	"testing"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

func TestTemplates_GrafanaDisableConfiguration(t *testing.T) {
	cfg := &config.ModuleConfig{
		LintersSettings: config.LintersSettings{
			Templates: config.TemplatesSettings{
				GrafanaDashboards: config.GrafanaDashboardsExcludeList{
					Disable: true, // disable the rule completely
				},
			},
		},
	}

	errList := errors.NewLintRuleErrorsList()
	tracker := exclusions.NewExclusionTracker()
	linter := NewWithTracker(cfg, tracker, errList)

	// Test that the linter was created with the correct configuration
	if !linter.cfg.GrafanaDashboards.Disable {
		t.Error("Expected grafana-dashboards rule to be disabled")
	}

	// Test that the tracker was properly initialized
	if tracker == nil {
		t.Error("Expected tracker to be initialized")
	}
}
