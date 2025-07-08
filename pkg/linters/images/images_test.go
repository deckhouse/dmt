package images

import (
	"testing"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

func TestImages_PatchesDisableConfiguration(t *testing.T) {
	cfg := &config.ModuleConfig{
		LintersSettings: config.LintersSettings{
			Images: config.ImageSettings{
				Patches: config.PatchesRuleSettings{
					Disable: true, // отключаем правило полностью
				},
			},
		},
	}

	errList := errors.NewLintRuleErrorsList()
	tracker := exclusions.NewExclusionTracker()
	linter := NewWithTracker(cfg, errList, tracker)

	// Test that the linter was created with the correct configuration
	if !linter.cfg.Patches.Disable {
		t.Error("Expected patches rule to be disabled")
	}

	// Test that the tracker was properly initialized
	if tracker == nil {
		t.Error("Expected tracker to be initialized")
	}
}
