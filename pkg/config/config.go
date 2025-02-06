package config

import (
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

// RootConfig encapsulates the config data specified in the YAML config file.
type RootConfig struct {
	GlobalSettings global.Global `mapstructure:"global"`
}

type ModuleConfig struct {
	LintersSettings LintersSettings `mapstructure:"linters-settings"`
}

func assignIfNotEmpty(v, input *pkg.Level) {
	if input != nil {
		*v = *input
	}
}

func NewDefaultRootConfig(dirs []string) (*RootConfig, error) {
	cfg := &RootConfig{}

	if err := NewLoader(cfg, dirs...).Load(); err != nil {
		return nil, err
	}

	return cfg, nil
}
