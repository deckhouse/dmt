package config

import (
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/pkg/errors"
)

var (
	GlobalExcludes *LintersSettings
)

// Config encapsulates the config data specified in the YAML config file.
type Config struct {
	cfgDir string // The directory containing the config file.

	LintersSettings LintersSettings `mapstructure:"linters-settings"`
	WarningsOnly    []string        `mapstructure:"warnings-only"`
}

func NewDefault(dirs []string) (*Config, error) {
	cfg := &Config{}

	if err := NewLoader(cfg, dirs).Load(); err != nil {
		return nil, err
	}

	errors.WarningsOnly = cfg.WarningsOnly
	for _, w := range cfg.WarningsOnly {
		logger.InfoF("Linter %q is marked as warnings-only. It will not fail the pipeline", w)
	}

	return cfg, nil
}
