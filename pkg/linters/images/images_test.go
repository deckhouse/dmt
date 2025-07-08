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
					Disable: true, // disable the rule completely
				},
			},
		},
	}

	errList := errors.NewLintRuleErrorsList()
	tracker := exclusions.NewExclusionTracker()
	linter := NewWithTracker(cfg, tracker, errList)

	// Test that the linter was created with the correct configuration
	if !linter.cfg.Patches.Disable {
		t.Error("Expected patches rule to be disabled")
	}

	// Test that the tracker was properly initialized
	if tracker == nil {
		t.Error("Expected tracker to be initialized")
	}
}
