package config

type LintersSettings struct {
	OpenAPI    OpenAPISettings    `mapstructure:"openapi"`
	NoCyrillic NoCyrillicSettings `mapstructure:"nocyrillic"`
	Copyright  CopyrightSettings  `mapstructure:"copyright"`
	Probes     ProbesSettings     `mapstructure:"probes"`
	Matrix     MatrixSettings     `mapstructure:"matrix"`
}

type OpenAPISettings struct {
	// EnumFileExcludes contains map with key string contained module name and file path separated by :
	EnumFileExcludes       map[string][]string `mapstructure:"enum-file-excludes"`
	HAAbsoluteKeysExcludes map[string]string   `mapstructure:"ha-absolute-keys-excludes"`
	KeyBannedNames         []string            `mapstructure:"key-banned-names"`
}

type NoCyrillicSettings struct {
	NoCyrillicFileExcludes []string `mapstructure:"no-cyrillic-file-excludes"`
	FileExtensions         []string `mapstructure:"file-extensions"`
	SkipDocRe              string   `mapstructure:"skip-doc-re"`
	SkipI18NRe             string   `mapstructure:"skip-i18n-re"`
	SkipSelfRe             string   `mapstructure:"skip-self-re"`
}

type CopyrightSettings struct {
	CopyrightExcludes []string `mapstructure:"copyright-excludes"`
}

type ProbesSettings struct {
	ProbesExcludes map[string][]string `mapstructure:"probes-excludes"`
}

type MatrixSettings struct {
	SkipOssChecks      []string            `mapstructure:"skip-oss-checks"`
	SkipCheckWildcards map[string][]string `mapstructure:"skip-check-wildcards"`
}
