package module

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/module"
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
	linter := NewWithTracker(cfg, tracker, errList)

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
	linter := NewWithTracker(cfg, tracker, errList)

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
	err = os.WriteFile(filepath.Join(moduleDir, "module.yaml"), []byte(moduleYaml), 0600)
	require.NoError(t, err)

	// Create Chart.yaml
	chartYaml := `name: test-module
version: 1.0.0`
	err = os.WriteFile(filepath.Join(moduleDir, "Chart.yaml"), []byte(chartYaml), 0600)
	require.NoError(t, err)

	// Create openapi directory and minimal schema files
	openAPIDir := filepath.Join(moduleDir, "openapi")
	err = os.MkdirAll(openAPIDir, 0755)
	require.NoError(t, err)

	// Create minimal config-values.yaml
	configValuesYaml := `type: object
properties: {}`
	err = os.WriteFile(filepath.Join(openAPIDir, "config-values.yaml"), []byte(configValuesYaml), 0600)
	require.NoError(t, err)

	// Create minimal values.yaml
	valuesYaml := `type: object
properties: {}`
	err = os.WriteFile(filepath.Join(openAPIDir, "values.yaml"), []byte(valuesYaml), 0600)
	require.NoError(t, err)

	// Create a .go file (will be processed by license linter)
	goFile := filepath.Join(moduleDir, "main.go")
	err = os.WriteFile(goFile, []byte("package main\n\nfunc main() {}\n"), 0600)
	require.NoError(t, err)

	// Create a binary file (will NOT be processed by license linter because it doesn't match fileToCheckRe)
	binaryDir := filepath.Join(moduleDir, "images", "simple-bridge", "src", "rootfs", "bin")
	err = os.MkdirAll(binaryDir, 0755)
	require.NoError(t, err)
	binaryFile := filepath.Join(binaryDir, "simple-bridge")
	err = os.WriteFile(binaryFile, []byte("binary content"), 0600)
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
	linter := NewWithTracker(cfg, tracker, errorList)

	// Create module using NewModule function
	mod, err := module.NewModule(moduleDir, nil, nil, errorList)
	require.NoError(t, err)

	// Run linter
	linter.Run(mod)

	// Check unused exclusions
	unused := tracker.GetUnusedExclusions()

	// Debug: print all unused exclusions
	t.Logf("All unused exclusions: %+v", unused)

	// The binary file exclusion не должен появляться в unused, потому что он не подходит под fileToCheckRe
	// The main.go exclusion должен быть использован
	if unused["module"]["license"] != nil && len(unused["module"]["license"]) > 0 {
		t.Errorf("Expected no unused exclusions, got: %+v", unused["module"]["license"])
	}
}
