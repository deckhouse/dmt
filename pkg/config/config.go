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

func (cfg *LintersSettings) MergeGlobal(lcfg *global.Linters) {
	cfg.OpenAPI.Impact = lcfg.OpenAPI.Impact
	cfg.NoCyrillic.Impact = lcfg.NoCyrillic.Impact
	cfg.License.Impact = lcfg.License.Impact
	cfg.OSS.Impact = lcfg.OSS.Impact
	cfg.Probes.Impact = lcfg.Probes.Impact
	cfg.Container.Impact = lcfg.Container.Impact
	cfg.K8SResources.Impact = lcfg.K8SResources.Impact
	cfg.VPAResources.Impact = lcfg.VPAResources.Impact
	cfg.PDBResources.Impact = lcfg.PDBResources.Impact
	cfg.CRDResources.Impact = lcfg.CRDResources.Impact
	cfg.Images.Impact = lcfg.Images.Impact
	cfg.Rbac.Impact = lcfg.Rbac.Impact
	cfg.Resources.Impact = lcfg.Resources.Impact
	cfg.Monitoring.Impact = lcfg.Monitoring.Impact
	cfg.Ingress.Impact = lcfg.Ingress.Impact
	cfg.Module.Impact = lcfg.Module.Impact
	cfg.Conversions.Impact = lcfg.Conversions.Impact
}

func NewDefaultRootConfig(dirs []string) (*RootConfig, error) {
	cfg := &RootConfig{}

	if err := NewLoader(cfg, dirs...).Load(); err != nil {
		return nil, err
	}

	return cfg, nil
}
