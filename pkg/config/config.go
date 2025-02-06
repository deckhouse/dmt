package config

import (
	"github.com/deckhouse/dmt/pkg/config/global"
)

// RootConfig encapsulates the config data specified in the YAML config file.
type RootConfig struct {
	GlobalSettings global.Global `mapstructure:"global"`
}

type ModuleConfig struct {
	LintersSettings LintersSettings `mapstructure:"linters-settings"`
}

func (cfg *LintersSettings) MergeGlobal(gcfg *global.Global) {
	cfg.OpenAPI.Impact = gcfg.Linters.Openapi.Impact
	cfg.NoCyrillic.Impact = gcfg.Linters.NoCyrillic.Impact
	cfg.License.Impact = gcfg.Linters.License.Impact
	cfg.OSS.Impact = gcfg.Linters.OSS.Impact
	cfg.Probes.Impact = gcfg.Linters.Probes.Impact
	cfg.Container.Impact = gcfg.Linters.Container.Impact
	cfg.K8SResources.Impact = gcfg
	cfg.CRDResources.Impact = gcfg.Linters.CRD.Impact
}

func NewDefaultRootConfig(dirs []string) (*RootConfig, error) {
	cfg := &RootConfig{}

	if err := NewLoader(cfg, dirs...).Load(); err != nil {
		return nil, err
	}

	return cfg, nil
}
