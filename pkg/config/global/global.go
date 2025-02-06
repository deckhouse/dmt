package global

import "github.com/deckhouse/dmt/pkg"

type Global struct {
	Linters Linters `mapstructure:"linters"`
}

type Linters struct {
	Openapi      LinterConfig `mapstructure:"openapi"`
	NoCyrillic   LinterConfig `mapstructure:"nocyrillic"`
	License      LinterConfig `mapstructure:"license"`
	OSS          LinterConfig `mapstructure:"oss"`
	Probes       LinterConfig `mapstructure:"probes"`
	Container    LinterConfig `mapstructure:"container"`
	K8SResources LinterConfig `mapstructure:"rbacproxy"`
	VPA          LinterConfig `mapstructure:"vpa"`
	PDB          LinterConfig `mapstructure:"pdb"`
	CRD          LinterConfig `mapstructure:"crd"`
	Images       LinterConfig `mapstructure:"images"`
	RBAC         LinterConfig `mapstructure:"rbac"`
	Monitoring   LinterConfig `mapstructure:"monitoring"`
	Ingress      LinterConfig `mapstructure:"ingress"`
	Module       LinterConfig `mapstructure:"module"`
	Conversions  LinterConfig `mapstructure:"conversions"`
}

type LinterConfig struct {
	Impact pkg.Level `mapstructure:"impact" default:"critical"`
}
