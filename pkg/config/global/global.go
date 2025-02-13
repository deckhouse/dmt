package global

import "github.com/deckhouse/dmt/pkg"

type Global struct {
	Linters Linters `mapstructure:"linters-settings"`
}

type Linters struct {
	OpenAPI       LinterConfig `mapstructure:"openapi"`
	NoCyrillic    LinterConfig `mapstructure:"nocyrillic"`
	License       LinterConfig `mapstructure:"license"`
	Container     LinterConfig `mapstructure:"container"`
	KubeRBACProxy LinterConfig `mapstructure:"kube-rbac-proxy"`
	CRDResources  LinterConfig `mapstructure:"crd_resources"`
	Images        LinterConfig `mapstructure:"images"`
	Rbac          LinterConfig `mapstructure:"rbac"`
	Resources     LinterConfig `mapstructure:"resources"`
	Monitoring    LinterConfig `mapstructure:"monitoring"`
	Ingress       LinterConfig `mapstructure:"ingress"`
	Module        LinterConfig `mapstructure:"module"`
	Templates     LinterConfig `mapstructure:"templates"`
}

type LinterConfig struct {
	Impact *pkg.Level `mapstructure:"impact"`
}
