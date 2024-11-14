package config

type LintersSettings struct {
	OpenAPI      OpenAPISettings      `mapstructure:"openapi"`
	NoCyrillic   NoCyrillicSettings   `mapstructure:"nocyrillic"`
	License      LicenseSettings      `mapstructure:"license"`
	Probes       ProbesSettings       `mapstructure:"probes"`
	Container    ContainerSettings    `mapstructure:"container"`
	K8SResources K8SResourcesSettings `mapstructure:"k8s_resources"`
	Helm         HelmSettings         `mapstructure:"helm"`
	Rbac         RbacSettings         `mapstructure:"rbac"`
	Resources    ResourcesSettings    `mapstructure:"resources"`
	Monitoring   MonitoringSettings   `mapstructure:"monitoring"`
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

type LicenseSettings struct {
	CopyrightExcludes []string `mapstructure:"copyright-excludes"`
	SkipOssChecks     []string `mapstructure:"skip-oss-checks"`
}

type ProbesSettings struct {
	ProbesExcludes map[string][]string `mapstructure:"probes-excludes"`
}

type ContainerSettings struct {
	SkipContainers []string `mapstructure:"skip-containers"`
}

type K8SResourcesSettings struct {
	SkipKubeRbacProxyChecks []string `mapstructure:"skip-kube-rbac-proxy-checks"`
	SkipContainerChecks     []string `mapstructure:"skip-container-checks"`
	SkipVPAChecks           []string `mapstructure:"skip-vpa-checks"`
	SkipPDBChecks           []string `mapstructure:"skip-pdb-checks"`
}

type ResourcesSettings struct{}

type MonitoringSettings struct {
	SkipModuleChecks []string `mapstructure:"skip-module-checks"`
}

type RbacSettings struct {
	SkipCheckWildcards     map[string][]string `mapstructure:"skip-check-wildcards"`
	SkipModuleCheckBinding []string            `mapstructure:"skip-module-check-binding"`
	SkipObjectCheckBinding []string            `mapstructure:"skip-object-check-binding"`
}

type HelmSettings struct {
	SkipModuleImageName      []string `mapstructure:"skip-module-image-name"`
	SkipDistrolessImageCheck []string `mapstructure:"skip-distroless-image-check"`
}
