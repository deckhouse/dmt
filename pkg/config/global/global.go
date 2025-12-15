/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package global

import "github.com/deckhouse/dmt/pkg"

type Global struct {
	Linters Linters `mapstructure:"linters-settings"`
}

type Linters struct {
	Container     ContainerLinterConfig     `mapstructure:"container"`
	Hooks         LinterConfig              `mapstructure:"hooks"`
	Images        ImagesLinterConfig        `mapstructure:"images"`
	License       LinterConfig              `mapstructure:"license"`
	Module        ModuleLinterConfig        `mapstructure:"module"`
	NoCyrillic    LinterConfig              `mapstructure:"no-cyrillic"`
	OpenAPI       LinterConfig              `mapstructure:"openapi"`
	Rbac          LinterConfig              `mapstructure:"rbac"`
	Templates     TemplatesLinterConfig     `mapstructure:"templates"`
	Documentation DocumentationLinterConfig `mapstructure:"documentation"`
}

type LinterConfig struct {
	Impact string `mapstructure:"impact"`
}

type ContainerLinterConfig struct {
	LinterConfig `mapstructure:",squash"`
	Rules        ContainerRules `mapstructure:"rules"`
}

type ContainerRules struct {
	RecommendedLabelsRule         RuleConfig `mapstructure:"recommended-labels"`
	NamespaceLabelsRule           RuleConfig `mapstructure:"namespace-labels"`
	ApiVersionRule                RuleConfig `mapstructure:"api-version"`
	PriorityClassRule             RuleConfig `mapstructure:"priority-class"`
	DNSPolicyRule                 RuleConfig `mapstructure:"dns-policy"`
	ControllerSecurityContextRule RuleConfig `mapstructure:"controller-security-context"`
	NewRevisionHistoryLimitRule   RuleConfig `mapstructure:"revision-history-limit"`

	// Container-specific rules
	NameDuplicatesRule           RuleConfig `mapstructure:"name-duplicates"`
	ReadOnlyRootFilesystemRule   RuleConfig `mapstructure:"read-only-root-filesystem"`
	NoNewPrivilegesRule          RuleConfig `mapstructure:"no-new-privileges"`
	SeccompProfileRule           RuleConfig `mapstructure:"seccomp-profile"`
	HostNetworkPortsRule         RuleConfig `mapstructure:"host-network-ports"`
	EnvVariablesDuplicatesRule   RuleConfig `mapstructure:"env-variables-duplicates"`
	ImageDigestRule              RuleConfig `mapstructure:"image-digest"`
	ImagePullPolicyRule          RuleConfig `mapstructure:"image-pull-policy"`
	ResourcesRule                RuleConfig `mapstructure:"resources"`
	ContainerSecurityContextRule RuleConfig `mapstructure:"container-security-context"`
	PortsRule                    RuleConfig `mapstructure:"ports"`
	LivenessRule                 RuleConfig `mapstructure:"liveness-probe"`
	ReadinessRule                RuleConfig `mapstructure:"readiness-probe"`
}

type ImagesLinterConfig struct {
	LinterConfig `mapstructure:",squash"`
	Rules        ImageRules `mapstructure:"rules"`
}

type ImageRules struct {
	DistrolessRule RuleConfig `mapstructure:"distroless"`
	ImageRule      RuleConfig `mapstructure:"image"`
	PatchesRule    RuleConfig `mapstructure:"patches"`
	WerfRule       RuleConfig `mapstructure:"werf"`
}

type RuleConfig struct {
	Impact string `mapstructure:"impact"`
}

type DocumentationLinterConfig struct {
	LinterConfig `mapstructure:",squash"`
	Rules        DocumentationRules `mapstructure:"rules"`
}

type DocumentationRules struct {
	BilingualRule          RuleConfig `mapstructure:"bilingual"`
	ReadmeRule             RuleConfig `mapstructure:"readme"`
	NoCyrillicExcludeRules RuleConfig `mapstructure:"cyrillic-in-english"`
	ChangelogRule          RuleConfig `mapstructure:"changelog"`
}

type ModuleLinterConfig struct {
	LinterConfig `mapstructure:",squash"`
	Rules        ModuleLinterRules `mapstructure:"rules"`
}

type ModuleLinterRules struct {
	DefinitionFileRule    RuleConfig `mapstructure:"definition-file"`
	OSSRule               RuleConfig `mapstructure:"oss"`
	ConversionRule        RuleConfig `mapstructure:"conversion"`
	HelmignoreRule        RuleConfig `mapstructure:"helmignore"`
	LicenseRule           RuleConfig `mapstructure:"license"`
	RequarementsRule      RuleConfig `mapstructure:"requarements"`
	LegacyReleaseFileRule RuleConfig `mapstructure:"legacy-release-file"`
}

type TemplatesLinterConfig struct {
	LinterConfig `mapstructure:",squash"`
	Rules        TemplatesLinterRules `mapstructure:"rules"`
}

type TemplatesLinterRules struct {
	VPARule           RuleConfig `mapstructure:"vpa"`
	PDBRule           RuleConfig `mapstructure:"pdb"`
	IngressRule       RuleConfig `mapstructure:"ingress"`
	PrometheusRule    RuleConfig `mapstructure:"prometheus-rules"`
	GrafanaRule       RuleConfig `mapstructure:"grafana-dashboards"`
	KubeRBACProxyRule RuleConfig `mapstructure:"kube-rbac-proxy"`
	ServicePortRule   RuleConfig `mapstructure:"service-port"`
	ClusterDomainRule RuleConfig `mapstructure:"cluster-domain"`
	RegistryRule      RuleConfig `mapstructure:"registry"`
}

func (c LinterConfig) IsWarn() bool {
	return c.Impact == pkg.Warn.String()
}
