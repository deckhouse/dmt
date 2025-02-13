package global

import "github.com/deckhouse/dmt/pkg"

type Global struct {
	Linters Linters `mapstructure:"linters-settings"`
}

type Linters struct {
	Container  LinterConfig `mapstructure:"container"`
	Hooks      LinterConfig `mapstructure:"hooks"`
	Images     LinterConfig `mapstructure:"images"`
	License    LinterConfig `mapstructure:"license"`
	Module     LinterConfig `mapstructure:"module"`
	NoCyrillic LinterConfig `mapstructure:"nocyrillic"`
	OpenAPI    LinterConfig `mapstructure:"openapi"`
	Rbac       LinterConfig `mapstructure:"rbac"`
	Templates  LinterConfig `mapstructure:"templates"`
}

type LinterConfig struct {
	Impact *pkg.Level `mapstructure:"impact"`
}
