package module

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/stretchr/testify/require"
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
					Disable: true, // disable the rule completely
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

func TestModule_LicenseExclusionTracking(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	moduleDir := filepath.Join(tempDir, "test-module")
	err := os.MkdirAll(moduleDir, 0755)
	require.NoError(t, err)

	// Create module.yaml
	moduleYaml := `name: test-module
namespace: test
version: 1.0.0`
	err = os.WriteFile(filepath.Join(moduleDir, "module.yaml"), []byte(moduleYaml), 0644)
	require.NoError(t, err)

	// Create a .go file (will be processed by license linter)
	goFile := filepath.Join(moduleDir, "main.go")
	err = os.WriteFile(goFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	require.NoError(t, err)

	// Create a binary file (will NOT be processed by license linter)
	binaryDir := filepath.Join(moduleDir, "images", "simple-bridge", "src", "rootfs", "bin")
	err = os.MkdirAll(binaryDir, 0755)
	require.NoError(t, err)
	binaryFile := filepath.Join(binaryDir, "simple-bridge")
	err = os.WriteFile(binaryFile, []byte("binary content"), 0755)
	require.NoError(t, err)

	// Create config with exclusions
	cfg := &config.ModuleConfig{
		LintersSettings: config.LintersSettings{
			Module: config.ModuleSettings{
				ExcludeRules: config.ModuleExcludeRules{
					License: config.LicenseExcludeRule{
						Files: config.StringRuleExcludeList{
							"images/simple-bridge/src/rootfs/bin/simple-bridge",
							"main.go",
						},
					},
				},
			},
		},
	}

	// Create error list and tracker
	errorList := errors.NewLintRuleErrorsList()
	tracker := exclusions.NewExclusionTracker()

	// Create module linter with tracking
	linter := NewWithTracker(cfg, errorList, tracker)

	// Create module using NewModule function
	mod, err := module.NewModule(moduleDir, nil, nil, errorList)
	require.NoError(t, err)

	// Run linter
	linter.Run(mod)

	// Check unused exclusions
	unused := tracker.GetUnusedExclusions()

	// The binary file exclusion should be marked as unused because it was never processed
	// The main.go exclusion should be marked as used because it was processed and excluded
	if len(unused["module"]["license"]) != 1 {
		t.Errorf("Expected 1 unused exclusion, got %d", len(unused["module"]["license"]))
	}

	if unused["module"]["license"][0] != "images/simple-bridge/src/rootfs/bin/simple-bridge" {
		t.Errorf("Expected unused exclusion to be 'images/simple-bridge/src/rootfs/bin/simple-bridge', got '%s'", unused["module"]["license"][0])
	}
}
