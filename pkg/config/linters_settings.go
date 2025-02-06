package config

import "github.com/deckhouse/dmt/pkg"

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

	Impact pkg.Level `mapstructure:"impact"`
}

type NoCyrillicSettings struct {
	NoCyrillicFileExcludes []string `mapstructure:"no-cyrillic-file-excludes"`

	Impact pkg.Level `mapstructure:"impact"`
}

type LicenseSettings struct {
	CopyrightExcludes []string `mapstructure:"copyright-excludes"`

	Impact pkg.Level `mapstructure:"impact"`
}

type OSSSettings struct {
	SkipOssChecks []string `mapstructure:"skip-oss-checks"`

	Impact pkg.Level `mapstructure:"impact"`
}

type ProbesSettings struct {
	ProbesExcludes map[string][]string `mapstructure:"probes-excludes"`

	Impact pkg.Level `mapstructure:"impact"`
}

type ContainerSettings struct {
	SkipContainers []string `mapstructure:"skip-containers"`

	Impact pkg.Level `mapstructure:"impact"`
}

type K8SResourcesSettings struct {
	SkipKubeRbacProxyChecks []string `mapstructure:"skip-kube-rbac-proxy-checks"`

	Impact pkg.Level `mapstructure:"impact"`
}

type VPAResourcesSettings struct {
	SkipVPAChecks []string `mapstructure:"skip-vpa-checks"`

	Impact pkg.Level `mapstructure:"impact"`
}

type PDBResourcesSettings struct {
	SkipPDBChecks []string `mapstructure:"skip-pdb-checks"`

	Impact pkg.Level `mapstructure:"impact"`
}

type CRDResourcesSettings struct {
	Impact pkg.Level `mapstructure:"impact"`
}

type ResourcesSettings struct {
	Impact pkg.Level `mapstructure:"impact"`
}

type MonitoringSettings struct {
	SkipModuleChecks []string `mapstructure:"skip-module-checks"`

	Impact pkg.Level `mapstructure:"impact"`
}

type RbacSettings struct {
	SkipCheckWildcards     map[string][]string `mapstructure:"skip-check-wildcards"`
	SkipModuleCheckBinding []string            `mapstructure:"skip-module-check-binding"`
	SkipObjectCheckBinding []string            `mapstructure:"skip-object-check-binding"`

	Impact pkg.Level `mapstructure:"impact"`
}

type ImageSettings struct {
	SkipModuleImageName      []string `mapstructure:"skip-module-image-name"`
	SkipDistrolessImageCheck []string `mapstructure:"skip-distroless-image-check"`
	SkipNamespaceCheck       []string `mapstructure:"skip-namespace-check"`

	Impact pkg.Level `mapstructure:"impact"`
}

type IngressSettings struct {
	SkipIngressChecks []string `mapstructure:"skip-ingress-checks"`

	Impact pkg.Level `mapstructure:"impact"`
}

type ModuleSettings struct {
	SkipCheckModuleYaml []string `mapstructure:"skip-check-module-yaml"`

	Impact pkg.Level `mapstructure:"impact"`
}

type ConversionsSettings struct {
	// skip all conversion checks for this modules
	SkipCheckModule []string `mapstructure:"skip-check"`
	// first conversion version to make conversion flow
	FirstVersion int `mapstructure:"first-version"`

	Impact pkg.Level `mapstructure:"impact"`
}
