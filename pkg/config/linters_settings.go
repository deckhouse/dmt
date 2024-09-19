package config

import (
	"errors"
	"fmt"
)

var defaultLintersSettings = LintersSettings{
	OpenAPI: OpenAPISettings{
		// EnumFileExcludes contains map with key string contained module name and file path separated by :
		EnumFileExcludes: map[string][]string{
			// all files
			"*": {"apiVersions[*].openAPISpec.properties.apiVersion"},
			"user-authn-crd:/crds/dex-provider.yaml": {
				// v1alpha1 migrated to v1
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.github.properties.teamNameField",
			},
			"prometheus-crd:/crds/grafanaadditionaldatasources.yaml": {
				// v1alpha1 migrated to v1
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.access",
			},
			"admission-policy-engine:/crds/operation-policy.yaml": {
				// probes are inherited from Kubernetes
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredProbes.items",
				// requests and limits are cpu and memory, they are taken from kubernetes
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredResources.properties.requests.items",
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredResources.properties.limits.items",
			},
			"admission-policy-engine:/crds/security-policy.yaml": {
				// volumes are inherited from kubernetes
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.allowedVolumes.items",
				// capabilities names are hardcoded, it's not ours
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.allowedCapabilities.items",
				"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredDropCapabilities.items",
			},
			"admission-policy-engine:/openapi/values.yaml": {
				// enforcement actions are discovered from label values and should be propagated further into the helm chart as is
				"properties.internal.properties.podSecurityStandards.properties.enforcementActions.items",
			},
			"cloud-provider-azure:/openapi/config-values.yaml": {
				// ignore Azure disk types
				"properties.storageClass.properties.provision.items.properties.type",
				"properties.storageClass.properties.provision.items.oneOf[*].properties.type",
			},
			"cloud-provider-aws:/openapi/config-values.yaml": {
				// ignore AWS disk types
				"properties.storageClass.properties.provision.items.properties.type",
				"properties.storageClass.properties.provision.items.oneOf[*].properties.type",
			},
			"cloud-provider-openstack:/openapi/values.yaml": {
				// ignore internal values
				"properties.internal.properties.discoveryData.properties.apiVersion",
			},
			"cloud-provider-aws:/openapi/values.yaml": {
				// ignore AWS disk types
				"properties.internal.properties.storageClasses.items.oneOf[*].properties.type",
			},
			"cloud-provider-vsphere:/openapi/config-values.yaml": {
				// ignore temporary flag that is already used (will be deleted after all CSIs are migrated)
				"properties.storageClass.properties.compatibilityFlag",
			},
			"cloud-provider-vsphere:/openapi/values.yaml": {
				// ignore internal values
				"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
				"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			},
			"cloud-provider-vcd:/openapi/values.yaml": {
				// ignore internal values
				"properties.internal.properties.discoveryData.properties.apiVersion",
				"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
				"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			},
			"cloud-provider-zvirt:/openapi/values.yaml": {
				// ignore internal values
				"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
				"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			},
			"cloud-provider-yandex:/openapi/values.yaml": {
				// ignore internal values
				"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
				"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
				"properties.internal.properties.providerClusterConfiguration.properties.zones.items",
				"properties.internal.properties.providerClusterConfiguration.properties.nodeGroups.items.properties.zones.items",
				"properties.internal.properties.providerClusterConfiguration.properties.masterNodeGroup.properties.zones.items",
			},
			"cni-flannel:/openapi/values.yaml": {
				// ignore internal values
				"properties.internal.properties.podNetworkMode",
			},
			"node-manager:/openapi/config-values.yaml": {
				// ignore internal values
				"properties.allowedBundles.items",
			},
			"kube-dns:/openapi/values.yaml": {
				// ignore internal values
				"properties.internal.properties.specificNodeType",
			},
			"prometheus:/openapi/values.yaml": {
				// grafana constant in internal values
				"properties.internal.properties.grafana.properties.alertsChannelsConfig.properties.notifiers.items.properties.type",
			},
			"ingress-nginx:/crds/ingress-nginx.yaml": {
				// GeoIP base constants: GeoIP2-ISP, GeoIP2-ASN, ...
				"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.geoIP2.properties.maxmindEditionIDs.items",
			},
			"ceph-csi:/crds/cephcsi.yaml": {
				// ignore file system names: ext4, xfs, etc.
				"properties.internal.properties.crs.items.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
				"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
			},
			"ceph-csi:/openapi/values.yaml": {
				// ignore file system names: ext4, xfs, etc.
				"properties.internal.properties.crs.items.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
				"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
			},
			"metallb:/openapi/config-values.yaml": {
				// ignore enum values
				"properties.addressPools.items.properties.protocol",
			},
		},
		HAAbsoluteKeysExcludes: map[string]string{
			"user-authn:/openapi/config-values.yaml": "properties.publishAPI.properties.https",
		},
		KeyBannedNames: []string{"x-examples", "examples", "example"},
	},
	NoCyrillic: NoCyrillicSettings{
		NoCyrillicFileExcludes: map[string]struct{}{
			"deckhouse:/oss.yaml":                                      {},
			"user-authn:/images/dex/web/templates/approval.html":       {},
			"user-authn:/images/dex/web/templates/device.html":         {},
			"user-authn:/images/dex/web/templates/device_success.html": {},
			"user-authn:/images/dex/web/templates/login.html":          {},
			"user-authn:/images/dex/web/templates/oob.html":            {},
			"user-authn:/images/dex/web/templates/password.html":       {},
			"documentation:/images/web/modules-docs/hugo.yaml":         {},
			"documentation:/images/web/site/_data/topnav.yml":          {},
		},
		FileExtensions: []string{".yaml", ".yml", ".md", ".txt", ".go", ".sh", ".html"},
		SkipSelfRe:     `no_cyrillic(_test)?.go$`,
		SkipDocRe:      `doc-ru-.+\.ya?ml$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+`,
		SkipI18NRe:     `/i18n/`,
	},
	Copyright: CopyrightSettings{
		CopyrightExcludes: map[string]struct{}{},
	},
	Probes: ProbesSettings{
		ProbesExcludes: map[string]map[string]struct{}{
			"d8-system": {
				"handler":         {},
				"deckhouse":       {},
				"kube-rbac-proxy": {},
				"kube-router":     {},
				"web":             {},
			},
			"d8-admission-policy-engine": {
				"constraint-exporter": {},
				"kube-rbac-proxy":     {},
			},
			"d8-cni-cilium": {
				"kube-rbac-proxy":          {},
				"operator":                 {},
				"pause-cilium":             {},
				"pause-check-linux-kernel": {},
				"pause-kube-rbac-proxy":    {},
				"frontend":                 {},
				"backend":                  {},
			},
			"d8-cloud-provider-aws": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
			},
			"d8-cloud-provider-azure": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
			},
			"d8-cloud-provider-gcp": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
			},
			"d8-cloud-provider-yandex": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
			},
			"d8-local-path-provisioner": {
				"local-path-provisioner": {},
			},
			"d8-cni-flannel": {
				"kube-flannel": {},
			},
			"d8-cni-simple-bridge": {
				"simple-bridge": {},
			},
			"kube-system": {
				"kube-proxy":                              {},
				"kube-rbac-proxy":                         {},
				"image-holder-etcd":                       {},
				"image-holder-kube-apiserver129":          {},
				"image-holder-kube-apiserver-healthcheck": {},
				"image-holder-kube-controller-manager129": {},
				"image-holder-kube-scheduler129":          {},
				"admission-controller":                    {},
				"recommender":                             {},
				"updater":                                 {},
				"iptables-loop":                           {},
			},
			"d8-snapshot-controller": {
				"snapshot-controller": {},
				"kube-rbac-proxy":     {},
				"snapshot-validation": {},
			},
			"d8-cert-manager": {
				"cainjector":      {},
				"kube-rbac-proxy": {},
				"cert-manager":    {},
			},
			"d8-istio": {
				"kube-rbac-proxy": {},
				"operator":        {},
			},
			"test-namespace": {
				"redis": {},
			},
			"dex-authenticator-namespace": {
				"redis": {},
			},
			"d8-operator-prometheus": {
				"prometheus-operator": {},
				"kube-rbac-proxy":     {},
			},
			"d8-monitoring": {
				"grafana":                              {},
				"dashboard-provisioner":                {},
				"kube-rbac-proxy":                      {},
				"exporter":                             {},
				"prometheus-metrics-adapter":           {},
				"image-availability-exporter":          {},
				"node-exporter":                        {},
				"kubelet-eviction-thresholds-exporter": {},
				"monitoring-ping":                      {},
			},
			"d8-ingress-nginx": {
				"kruise": {},
			},
			"d8-log-shipper": {
				"vector-reloader": {},
			},
			"d8-pod-reloader": {
				"kube-rbac-proxy": {},
			},
			"d8-chrony": {
				"chrony": {},
			},
			"d8-okmeter": {
				"okagent": {},
			},
			"d8-openvpn": {
				"openvpn-tcp":     {},
				"ovpn-admin":      {},
				"kube-rbac-proxy": {},
			},
			"d8-upmeter": {
				"agent":           {},
				"kube-rbac-proxy": {},
			},
			"d8-metallb": {
				"controller":      {},
				"kube-rbac-proxy": {},
			},
			"d8-delivery": {
				"redis":                            {},
				"werf-argocd-cmp-sidecar":          {},
				"argocd-applicationset-controller": {},
				"argocd-notifications-controller":  {},
				"argocd-application-controller":    {},
			},
			"d8-static-routing-manager": {
				"agent": {},
			},
			"d8-cloud-provider-openstack": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
			},
			"d8-cloud-provider-vcd": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
			},
			"d8-cloud-provider-vsphere": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
				"syncer":                   {},
			},
			"d8-cloud-provider-zvirt": {
				"node-driver-registrar":    {},
				"node":                     {},
				"kube-rbac-proxy":          {},
				"node-termination-handler": {},
				"provisioner":              {},
				"attacher":                 {},
				"resizer":                  {},
				"snapshotter":              {},
				"livenessprobe":            {},
				"controller":               {},
			},
			"d8-l2-load-balancer": {
				"kube-rbac-proxy": {},
				"controller":      {},
			},
			"d8-network-gateway": {
				"dhcp": {},
				"snat": {},
			},
			"d8-operator-trivy": {
				"kube-rbac-proxy": {},
				"report-updater":  {},
			},
			"d8-runtime-audit-engine": {
				"rules-loader": {},
			},
			"d8-dashboard": {
				"dashboard":       {},
				"metrics-scraper": {},
			},
		},
	},
}

type LintersSettings struct {
	OpenAPI    OpenAPISettings
	NoCyrillic NoCyrillicSettings
	Copyright  CopyrightSettings
	Probes     ProbesSettings
	Custom     map[string]CustomLinterSettings
}

func (s *LintersSettings) Validate() error {
	for name, settings := range s.Custom {
		if err := settings.Validate(); err != nil {
			return fmt.Errorf("custom linter %q: %w", name, err)
		}
	}

	return nil
}

// CustomLinterSettings encapsulates the meta-data of a private linter.
type CustomLinterSettings struct {
	// Type plugin type.
	// It can be `goplugin` or `module`.
	Type string `mapstructure:"type"`

	// Path to a plugin *.so file that implements the private linter.
	// Only for Go plugin system.
	Path string

	// Description describes the purpose of the private linter.
	Description string
	// OriginalURL The URL containing the source code for the private linter.
	OriginalURL string `mapstructure:"original-url"`

	// Settings plugin settings only work with linterdb.PluginConstructor symbol.
	Settings any
}

func (s *CustomLinterSettings) Validate() error {
	if s.Type == "module" {
		if s.Path != "" {
			return errors.New("path not supported with module type")
		}

		return nil
	}

	if s.Path == "" {
		return errors.New("path is required")
	}

	return nil
}

type OpenAPISettings struct {
	// EnumFileExcludes contains map with key string contained module name and file path separated by :
	EnumFileExcludes       map[string][]string `mapstructure:"enum-file-excludes"`
	HAAbsoluteKeysExcludes map[string]string   `mapstructure:"ha-absolute-keys-excludes"`
	KeyBannedNames         []string            `mapstructure:"key-banned-names"`
}

type NoCyrillicSettings struct {
	NoCyrillicFileExcludes map[string]struct{} `mapstructure:"no-cyrillic-file-excludes"`
	FileExtensions         []string            `mapstructure:"file-extensions"`
	SkipDocRe              string              `mapstructure:"skip-doc-re"`
	SkipI18NRe             string              `mapstructure:"skip-i18n-re"`
	SkipSelfRe             string              `mapstructure:"skip-self-re"`
}

type CopyrightSettings struct {
	CopyrightExcludes map[string]struct{} `mapstructure:"copyright-excludes"`
}

type ProbesSettings struct {
	ProbesExcludes map[string]map[string]struct{} `mapstructure:"probes-excludes"`
}
