package config

import (
	"github.com/deckhouse/d8-lint/pkg/errors"
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

	return cfg, nil
}

func (c *Config) Validate() error {
	validators := []func() error{
		c.LintersSettings.Validate,
	}

	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}

	return nil
}
