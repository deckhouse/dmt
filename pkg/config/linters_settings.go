package config

type LintersSettings struct {
	OpenAPI    OpenAPISettings    `mapstructure:"openapi"`
	NoCyrillic NoCyrillicSettings `mapstructure:"nocyrillic"`
	Copyright  CopyrightSettings  `mapstructure:"copyright"`
	Probes     ProbesSettings     `mapstructure:"probes"`
	Container  ContainerSettings  `mapstructure:"container"`
	Object     ObjectSettings     `mapstructure:"object"`
	Modules    ModulesSettings    `mapstructure:"modules"`
	Rbac       RbacSettings       `mapstructure:"rbac"`
	Resources  ResourcesSettings  `mapstructure:"resources"`
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

type MatrixSettings struct{}

type ContainerSettings struct{}

type ObjectSettings struct{}

type ResourcesSettings struct{}

type RbacSettings struct {
	SkipCheckWildcards map[string][]string `mapstructure:"skip-check-wildcards"`
}

type ModulesSettings struct {
	SkipOssChecks            []string `mapstructure:"skip-oss-checks"`
	SkipModuleImageName      []string `mapstructure:"skip-module-image-name"`
	SkipDistrolessImageCheck []string `mapstructure:"skip-distroless-image-check"`
}
