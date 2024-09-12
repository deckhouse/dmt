package config

// Config encapsulates the config data specified in the golangci-lint YAML config file.
type Config struct {
	cfgDir string // The directory containing the golangci-lint config file.

	LintersSettings LintersSettings `mapstructure:"linters-settings"`
	Linters         Linters         `mapstructure:"linters"`
}

func NewDefault() *Config {
	return &Config{
		LintersSettings: defaultLintersSettings,
	}
}

func (c *Config) Validate() error {
	validators := []func() error{
		c.LintersSettings.Validate,
		c.Linters.Validate,
	}

	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}

	return nil
}