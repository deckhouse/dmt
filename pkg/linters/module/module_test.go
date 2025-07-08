package module

import (
	"testing"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

func TestModule_ConversionsExclusionConfiguration(t *testing.T) {
	cfg := &config.ModuleConfig{
		LintersSettings: config.LintersSettings{
			Module: config.ModuleSettings{
				ExcludeRules: config.ModuleExcludeRules{
					Conversions: config.ConversionsExcludeRules{
						Files: config.StringRuleExcludeList{
							"openapi/conversions/v2.yaml",
						},
					},
				},
			},
		},
	}

	errList := errors.NewLintRuleErrorsList()
	tracker := exclusions.NewExclusionTracker()
	linter := NewWithTracker(cfg, errList, tracker)

	// Test that the linter was created with the correct configuration
	if linter.cfg.ExcludeRules.Conversions.Files[0] != "openapi/conversions/v2.yaml" {
		t.Errorf("Expected exclusion file 'openapi/conversions/v2.yaml', but got: %s", 
			linter.cfg.ExcludeRules.Conversions.Files[0])
	}

	// Test that the tracker was properly initialized
	if tracker == nil {
		t.Error("Expected tracker to be initialized")
	}
}

func TestModule_ConversionsDisableConfiguration(t *testing.T) {
	cfg := &config.ModuleConfig{
		LintersSettings: config.LintersSettings{
			Module: config.ModuleSettings{
				Conversions: config.ConversionsRuleSettings{
					Disable: true, // отключаем правило полностью
				},
			},
		},
	}

	errList := errors.NewLintRuleErrorsList()
	tracker := exclusions.NewExclusionTracker()
	linter := NewWithTracker(cfg, errList, tracker)

	// Test that the linter was created with the correct configuration
	if !linter.cfg.Conversions.Disable {
		t.Error("Expected conversions rule to be disabled")
	}

	// Test that the tracker was properly initialized
	if tracker == nil {
		t.Error("Expected tracker to be initialized")
	}
}
