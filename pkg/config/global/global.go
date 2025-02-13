package global

import "github.com/deckhouse/dmt/pkg"

type Global struct {
	Linters Linters `mapstructure:"linters-settings"`
}

type Linters struct {
	OpenAPI    LinterConfig `mapstructure:"openapi"`
	NoCyrillic LinterConfig `mapstructure:"nocyrillic"`
	License    LinterConfig `mapstructure:"license"`
	Container  LinterConfig `mapstructure:"container"`
	Images     LinterConfig `mapstructure:"images"`
	Rbac       LinterConfig `mapstructure:"rbac"`
	Resources  LinterConfig `mapstructure:"resources"`
	Hooks      LinterConfig `mapstructure:"hooks"`
	Module     LinterConfig `mapstructure:"module"`
	Templates  LinterConfig `mapstructure:"templates"`
}

type LinterConfig struct {
	Impact *pkg.Level `mapstructure:"impact"`
}
