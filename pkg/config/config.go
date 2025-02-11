package config

import (
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Config encapsulates the config data specified in the YAML config file.
type Config struct {
	cfgDir string // The directory containing the config file.

	LintersSettings LintersSettings `mapstructure:"linters-settings"`
	WarningsOnly    []string        `mapstructure:"warnings-only"`
}

var Cfg *Config

func NewDefault(dirs []string) error {
	Cfg = &Config{}

	if err := NewLoader(Cfg, dirs).Load(); err != nil {
		return err
	}

	errors.WarningsOnly = Cfg.WarningsOnly
	for _, w := range Cfg.WarningsOnly {
		logger.InfoF("Linter %q is marked as warnings-only. It will not fail the pipeline", w)
	}

	return nil
}
