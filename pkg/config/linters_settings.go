package config

import (
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

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

func (cfg *LintersSettings) MergeGlobal(lcfg *global.Linters) {
	assignIfNotEmpty(&cfg.OpenAPI.Impact, lcfg.OpenAPI.Impact)
	assignIfNotEmpty(&cfg.NoCyrillic.Impact, lcfg.NoCyrillic.Impact)
	assignIfNotEmpty(&cfg.License.Impact, lcfg.License.Impact)
	assignIfNotEmpty(&cfg.OSS.Impact, lcfg.OSS.Impact)
	assignIfNotEmpty(&cfg.Probes.Impact, lcfg.Probes.Impact)
	assignIfNotEmpty(&cfg.Container.Impact, lcfg.Container.Impact)
	assignIfNotEmpty(&cfg.K8SResources.Impact, lcfg.K8SResources.Impact)
	assignIfNotEmpty(&cfg.VPAResources.Impact, lcfg.VPAResources.Impact)
	assignIfNotEmpty(&cfg.PDBResources.Impact, lcfg.PDBResources.Impact)
	assignIfNotEmpty(&cfg.CRDResources.Impact, lcfg.CRDResources.Impact)
	assignIfNotEmpty(&cfg.Images.Impact, lcfg.Images.Impact)
	assignIfNotEmpty(&cfg.Rbac.Impact, lcfg.Rbac.Impact)
	assignIfNotEmpty(&cfg.Resources.Impact, lcfg.Resources.Impact)
	assignIfNotEmpty(&cfg.Monitoring.Impact, lcfg.Monitoring.Impact)
	assignIfNotEmpty(&cfg.Ingress.Impact, lcfg.Ingress.Impact)
	assignIfNotEmpty(&cfg.Module.Impact, lcfg.Module.Impact)
	assignIfNotEmpty(&cfg.Conversions.Impact, lcfg.Conversions.Impact)
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
	MonitoringRules *bool `mapstructure:"monitoring-rules"`

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
