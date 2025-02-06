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

func (cfg *LintersSettings) MergeGlobal(lcfg *global.Linters) {
	assignIfNotEmpty(&cfg.OpenAPI.Impact, lcfg.OpenAPI.Impact)
	assignIfNotEmpty(&cfg.NoCyrillic.Impact, lcfg.NoCyrillic.Impact)
	assignIfNotEmpty(&cfg.License.Impact, lcfg.License.Impact)
	assignIfNotEmpty(&cfg.OSS.Impact, lcfg.OSS.Impact)
	assignIfNotEmpty(&cfg.Probes.Impact, lcfg.Probes.Impact)
	assignIfNotEmpty(&cfg.Container.Impact, lcfg.Container.Impact)
	assignIfNotEmpty(&cfg.K8SResources.Impact, lcfg.K8SResources.Impact)
	assignIfNotEmpty(&cfg.VPAResources.Impact, lcfg.VPAResources.Impact)
	assignIfNotEmpty(&cfg.PDBResources.Impact, lcfg.PDBResources.Impact)
	assignIfNotEmpty(&cfg.CRDResources.Impact, lcfg.CRDResources.Impact)
	assignIfNotEmpty(&cfg.Images.Impact, lcfg.Images.Impact)
	assignIfNotEmpty(&cfg.Rbac.Impact, lcfg.Rbac.Impact)
	assignIfNotEmpty(&cfg.Resources.Impact, lcfg.Resources.Impact)
	assignIfNotEmpty(&cfg.Monitoring.Impact, lcfg.Monitoring.Impact)
	assignIfNotEmpty(&cfg.Ingress.Impact, lcfg.Ingress.Impact)
	assignIfNotEmpty(&cfg.Module.Impact, lcfg.Module.Impact)
	assignIfNotEmpty(&cfg.Conversions.Impact, lcfg.Conversions.Impact)
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
