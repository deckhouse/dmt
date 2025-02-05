package config

type LintersSettings struct {
	OpenAPI      OpenAPISettings      `mapstructure:"openapi"`
	NoCyrillic   NoCyrillicSettings   `mapstructure:"nocyrillic"`
	License      LicenseSettings      `mapstructure:"license"`
	OSS          OSSSettings          `mapstructure:"oss"`
	Probes       ProbesSettings       `mapstructure:"probes"`
	Container    ContainerSettings    `mapstructure:"container"`
	K8SResources K8SResourcesSettings `mapstructure:"k8s_resources"`
	VPAResources VPAResourcesSettings `mapstructure:"vpa_resources"`
	PDBResources PDBResourcesSettings `mapstructure:"pdb_resources"`
	CRDResources CRDResourcesSettings `mapstructure:"crd_resources"`
	Images       ImageSettings        `mapstructure:"images"`
	Rbac         RbacSettings         `mapstructure:"rbac"`
	Resources    ResourcesSettings    `mapstructure:"resources"`
	Monitoring   MonitoringSettings   `mapstructure:"monitoring"`
	Ingress      IngressSettings      `mapstructure:"ingress"`
	Module       ModuleSettings       `mapstructure:"module"`
	Conversions  ConversionsSettings  `mapstructure:"conversions"`
}

type OpenAPISettings struct {
	// EnumFileExcludes contains map with key string contained module name and file path separated by :
	EnumFileExcludes       map[string][]string `mapstructure:"enum-file-excludes"`
	HAAbsoluteKeysExcludes map[string]string   `mapstructure:"ha-absolute-keys-excludes"`
	KeyBannedNames         []string            `mapstructure:"key-banned-names"`
}

type NoCyrillicSettings struct {
	NoCyrillicFileExcludes []string `mapstructure:"no-cyrillic-file-excludes"`
}

type LicenseSettings struct {
	CopyrightExcludes []string `mapstructure:"copyright-excludes"`
}

type OSSSettings struct {
	SkipOssChecks []string `mapstructure:"skip-oss-checks"`
}

type ProbesSettings struct {
	ProbesExcludes map[string][]string `mapstructure:"probes-excludes"`
}

type ContainerSettings struct {
	SkipContainers []string `mapstructure:"skip-containers"`
}

type K8SResourcesSettings struct {
	SkipKubeRbacProxyChecks []string `mapstructure:"skip-kube-rbac-proxy-checks"`
}

type VPAResourcesSettings struct {
	SkipVPAChecks []string `mapstructure:"skip-vpa-checks"`
}

type PDBResourcesSettings struct {
	SkipPDBChecks []string `mapstructure:"skip-pdb-checks"`
}

type CRDResourcesSettings struct{}

type ResourcesSettings struct{}

type MonitoringSettings struct {
	SkipModuleChecks []string `mapstructure:"skip-module-checks"`
}

type RbacSettings struct {
	SkipCheckWildcards     map[string][]string `mapstructure:"skip-check-wildcards"`
	SkipModuleCheckBinding []string            `mapstructure:"skip-module-check-binding"`
	SkipObjectCheckBinding []string            `mapstructure:"skip-object-check-binding"`
}

type ImageSettings struct {
	SkipModuleImageName      []string `mapstructure:"skip-module-image-name"`
	SkipDistrolessImageCheck []string `mapstructure:"skip-distroless-image-check"`
	SkipNamespaceCheck       []string `mapstructure:"skip-namespace-check"`
}

type IngressSettings struct {
	SkipIngressChecks []string `mapstructure:"skip-ingress-checks"`
}

type ModuleSettings struct {
	SkipCheckModuleYaml []string `mapstructure:"skip-check-module-yaml"`
}

type ConversionsSettings struct {
	// skip all conversion checks for this modules
	SkipCheckModule []string `mapstructure:"skip-check"`
	// first conversion version to make conversion flow
	FirstVersion int `mapstructure:"first-version"`
}
