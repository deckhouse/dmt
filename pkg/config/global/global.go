package global

import "github.com/deckhouse/dmt/pkg"

type Global struct {
	Linters Linters `mapstructure:"linters-settings"`
}

type Linters struct {
	OpenAPI      LinterConfig `mapstructure:"openapi"`
	NoCyrillic   LinterConfig `mapstructure:"nocyrillic"`
	License      LinterConfig `mapstructure:"license"`
	Probes       LinterConfig `mapstructure:"probes"`
	Container    LinterConfig `mapstructure:"container"`
	K8SResources LinterConfig `mapstructure:"k8s_resources"`
	VPAResources LinterConfig `mapstructure:"vpa_resources"`
	PDBResources LinterConfig `mapstructure:"pdb_resources"`
	CRDResources LinterConfig `mapstructure:"crd_resources"`
	Images       LinterConfig `mapstructure:"images"`
	Rbac         LinterConfig `mapstructure:"rbac"`
	Resources    LinterConfig `mapstructure:"resources"`
	Monitoring   LinterConfig `mapstructure:"monitoring"`
	Ingress      LinterConfig `mapstructure:"ingress"`
	Module       LinterConfig `mapstructure:"module"`
}

type LinterConfig struct {
	Impact *pkg.Level `mapstructure:"impact"`
}
