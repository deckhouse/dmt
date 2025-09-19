/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apa	linterSettings.NoCyrillic.ExcludeRules.Files = pkg.StringRuleExcludeList(configSettings.NoCyrillic.NoCyrillicExcludeRules.Files)
	linterSettings.NoCyrillic.ExcludeRules.Directories = pkg.PrefixRuleExcludeList(configSettings.NoCyrillic.NoCyrillicExcludeRules.Directories)e.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package module

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/spec"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/internal/values"
	"github.com/deckhouse/dmt/internal/werf"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
	dmtErrors "github.com/deckhouse/dmt/pkg/errors"
)

const (
	ChartConfigFilename  = "Chart.yaml"
	ModuleConfigFilename = "module.yaml"
)

type Module struct {
	name        string
	namespace   string
	path        string
	chart       *chart.Chart
	objectStore *storage.UnstructuredObjectStore
	werfFile    string

	linterConfig *pkg.LintersSettings
}

type ModuleList []*Module

type ModuleYaml struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type ChartYaml struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (m *Module) String() string {
	return fmt.Sprintf("{Name: %s, Namespace: %s, Path: %s}", m.name, m.namespace, m.path)
}

func (m *Module) GetName() string {
	if m == nil {
		return ""
	}

	return m.name
}

func (m *Module) GetNamespace() string {
	if m == nil {
		return ""
	}
	return m.namespace
}

func (m *Module) GetPath() string {
	if m == nil {
		return ""
	}
	return m.path
}

func (m *Module) GetChart() *chart.Chart {
	if m == nil {
		return nil
	}
	return m.chart
}

func (m *Module) GetMetadata() *chart.Metadata {
	if m.chart == nil || m.chart.Metadata == nil {
		return nil
	}

	return m.chart.Metadata
}

func (m *Module) GetObjectStore() *storage.UnstructuredObjectStore {
	if m == nil {
		return nil
	}
	return m.objectStore
}

func (m *Module) GetStorage() map[storage.ResourceIndex]storage.StoreObject {
	if m == nil || m.objectStore == nil {
		return nil
	}
	return m.objectStore.Storage
}

func (m *Module) GetWerfFile() string {
	if m == nil {
		return ""
	}
	return m.werfFile
}

func (m *Module) GetModuleConfig() *pkg.LintersSettings {
	if m == nil {
		return nil
	}
	return m.linterConfig
}

func remapLinterSettings(configSettings *config.LintersSettings, globalConfig *global.Linters) *pkg.LintersSettings {
	linterSettings := &pkg.LintersSettings{}

	// Set linter-level impact levels
	linterSettings.Container.SetLevel(configSettings.Container.Impact)
	linterSettings.Image.SetLevel(configSettings.Images.Impact)
	linterSettings.NoCyrillic.SetLevel(configSettings.NoCyrillic.Impact)
	linterSettings.OpenAPI.SetLevel(configSettings.OpenAPI.Impact)
	linterSettings.Templates.SetLevel(configSettings.Templates.Impact)
	linterSettings.RBAC.SetLevel(configSettings.Rbac.Impact)
	linterSettings.Hooks.SetLevel(configSettings.Hooks.Impact)
	linterSettings.Module.SetLevel(configSettings.Module.Impact)

	// Container linter rules
	linterSettings.Container.Rules.RecommendedLabelsRule.SetLevel(globalConfig.Container.RecommendedLabelsRule.Impact, configSettings.Container.Impact)

	// Image linter rules
	linterSettings.Image.Rules.DistrolessRule.SetLevel(globalConfig.Images.Rules.DistrolessRule.Impact, configSettings.Images.Impact)
	linterSettings.Image.Rules.ImageRule.SetLevel(globalConfig.Images.Rules.ImageRule.Impact, configSettings.Images.Impact)
	linterSettings.Image.Rules.PatchesRule.SetLevel(globalConfig.Images.Rules.PatchesRule.Impact, configSettings.Images.Impact)
	linterSettings.Image.Rules.WerfRule.SetLevel(globalConfig.Images.Rules.WerfRule.Impact, configSettings.Images.Impact)

	// NoCyrillic linter rules
	linterSettings.NoCyrillic.Rules.NoCyrillicRule.SetLevel("", configSettings.NoCyrillic.Impact)

	// OpenAPI linter rules
	linterSettings.OpenAPI.Rules.EnumRule.SetLevel("", configSettings.OpenAPI.Impact)
	linterSettings.OpenAPI.Rules.HARule.SetLevel("", configSettings.OpenAPI.Impact)
	linterSettings.OpenAPI.Rules.CRDsRule.SetLevel("", configSettings.OpenAPI.Impact)
	linterSettings.OpenAPI.Rules.KeysRule.SetLevel("", configSettings.OpenAPI.Impact)

	// Templates linter rules
	linterSettings.Templates.Rules.VPARule.SetLevel("", configSettings.Templates.Impact)
	linterSettings.Templates.Rules.PDBRule.SetLevel("", configSettings.Templates.Impact)
	linterSettings.Templates.Rules.IngressRule.SetLevel("", configSettings.Templates.Impact)
	linterSettings.Templates.Rules.PrometheusRule.SetLevel("", configSettings.Templates.Impact)
	linterSettings.Templates.Rules.GrafanaRule.SetLevel("", configSettings.Templates.Impact)
	linterSettings.Templates.Rules.KubeRBACProxyRule.SetLevel("", configSettings.Templates.Impact)
	linterSettings.Templates.Rules.ServicePortRule.SetLevel("", configSettings.Templates.Impact)
	linterSettings.Templates.Rules.ClusterDomainRule.SetLevel("", configSettings.Templates.Impact)

	// RBAC linter rules
	linterSettings.RBAC.Rules.UserAuthRule.SetLevel("", configSettings.Rbac.Impact)
	linterSettings.RBAC.Rules.BindingRule.SetLevel("", configSettings.Rbac.Impact)
	linterSettings.RBAC.Rules.PlacementRule.SetLevel("", configSettings.Rbac.Impact)
	linterSettings.RBAC.Rules.WildcardsRule.SetLevel("", configSettings.Rbac.Impact)

	// Hooks linter rules
	linterSettings.Hooks.Rules.HooksRule.SetLevel("", configSettings.Hooks.Impact)

	// Module linter rules
	linterSettings.Module.Rules.DefinitionFileRule.SetLevel("", configSettings.Module.Impact)
	linterSettings.Module.Rules.OSSRule.SetLevel("", configSettings.Module.Impact)
	linterSettings.Module.Rules.ConversionRule.SetLevel("", configSettings.Module.Impact)
	linterSettings.Module.Rules.HelmignoreRule.SetLevel("", configSettings.Module.Impact)
	linterSettings.Module.Rules.LicenseRule.SetLevel("", configSettings.Module.Impact)
	linterSettings.Module.Rules.RequarementsRule.SetLevel("", configSettings.Module.Impact)

	// Exclude Rules Mapping
	// Container exclude rules
	linterSettings.Container.ExcludeRules.ControllerSecurityContext = configSettings.Container.ExcludeRules.ControllerSecurityContext.Get()
	linterSettings.Container.ExcludeRules.DNSPolicy = configSettings.Container.ExcludeRules.DNSPolicy.Get()
	linterSettings.Container.ExcludeRules.HostNetworkPorts = configSettings.Container.ExcludeRules.HostNetworkPorts.Get()
	linterSettings.Container.ExcludeRules.Ports = configSettings.Container.ExcludeRules.Ports.Get()
	linterSettings.Container.ExcludeRules.ReadOnlyRootFilesystem = configSettings.Container.ExcludeRules.ReadOnlyRootFilesystem.Get()
	linterSettings.Container.ExcludeRules.ImageDigest = configSettings.Container.ExcludeRules.ImageDigest.Get()
	linterSettings.Container.ExcludeRules.Resources = configSettings.Container.ExcludeRules.Resources.Get()
	linterSettings.Container.ExcludeRules.SecurityContext = configSettings.Container.ExcludeRules.SecurityContext.Get()
	linterSettings.Container.ExcludeRules.Liveness = configSettings.Container.ExcludeRules.Liveness.Get()
	linterSettings.Container.ExcludeRules.Readiness = configSettings.Container.ExcludeRules.Readiness.Get()
	linterSettings.Container.ExcludeRules.Description = pkg.StringRuleExcludeList(configSettings.Container.ExcludeRules.Description)

	// Image exclude rules
	linterSettings.Image.ExcludeRules.SkipImageFilePathPrefix = pkg.PrefixRuleExcludeList(configSettings.Images.ExcludeRules.SkipImageFilePathPrefix)
	linterSettings.Image.ExcludeRules.SkipDistrolessFilePathPrefix = pkg.PrefixRuleExcludeList(configSettings.Images.ExcludeRules.SkipDistrolessFilePathPrefix)

	// Image settings
	linterSettings.Image.Patches.Disable = configSettings.Images.Patches.Disable
	linterSettings.Image.Werf.Disable = configSettings.Images.Werf.Disable

	// NoCyrillic exclude rules
	linterSettings.NoCyrillic.ExcludeRules.Files = pkg.StringRuleExcludeList(configSettings.NoCyrillic.NoCyrillicExcludeRules.Files)
	linterSettings.NoCyrillic.ExcludeRules.Directories = pkg.PrefixRuleExcludeList(configSettings.NoCyrillic.NoCyrillicExcludeRules.Directories)

	// OpenAPI exclude rules
	linterSettings.OpenAPI.ExcludeRules.KeyBannedNames = configSettings.OpenAPI.OpenAPIExcludeRules.KeyBannedNames
	linterSettings.OpenAPI.ExcludeRules.EnumFileExcludes = configSettings.OpenAPI.OpenAPIExcludeRules.EnumFileExcludes
	linterSettings.OpenAPI.ExcludeRules.HAAbsoluteKeysExcludes = pkg.StringRuleExcludeList(configSettings.OpenAPI.OpenAPIExcludeRules.HAAbsoluteKeysExcludes)
	linterSettings.OpenAPI.ExcludeRules.CRDNamesExcludes = pkg.StringRuleExcludeList(configSettings.OpenAPI.OpenAPIExcludeRules.CRDNamesExcludes)

	// Templates exclude rules
	linterSettings.Templates.ExcludeRules.VPAAbsent = configSettings.Templates.ExcludeRules.VPAAbsent.Get()
	linterSettings.Templates.ExcludeRules.PDBAbsent = configSettings.Templates.ExcludeRules.PDBAbsent.Get()
	linterSettings.Templates.ExcludeRules.ServicePort = configSettings.Templates.ExcludeRules.ServicePort.Get()
	linterSettings.Templates.ExcludeRules.KubeRBACProxy = pkg.StringRuleExcludeList(configSettings.Templates.ExcludeRules.KubeRBACProxy)
	linterSettings.Templates.ExcludeRules.Ingress = configSettings.Templates.ExcludeRules.Ingress.Get()

	// Templates settings
	linterSettings.Templates.PrometheusRuleSettings.Disable = configSettings.Templates.PrometheusRules.Disable
	linterSettings.Templates.GrafanaDashboardsSettings.Disable = configSettings.Templates.GrafanaDashboards.Disable

	// RBAC exclude rules
	linterSettings.RBAC.ExcludeRules.BindingSubject = pkg.StringRuleExcludeList(configSettings.Rbac.ExcludeRules.BindingSubject)
	linterSettings.RBAC.ExcludeRules.Placement = configSettings.Rbac.ExcludeRules.Placement.Get()
	linterSettings.RBAC.ExcludeRules.Wildcards = configSettings.Rbac.ExcludeRules.Wildcards.Get()

	// Hooks settings
	linterSettings.Hooks.IngressRuleSettings.Disable = configSettings.Hooks.Ingress.Disable

	// Module exclude rules
	linterSettings.Module.ExcludeRules.License.Files = pkg.StringRuleExcludeList(configSettings.Module.ExcludeRules.License.Files)
	linterSettings.Module.ExcludeRules.License.Directories = pkg.PrefixRuleExcludeList(configSettings.Module.ExcludeRules.License.Directories)

	// Module settings
	linterSettings.Module.OSSRuleSettings.Disable = configSettings.Module.OSS.Disable
	linterSettings.Module.DefinitionFileRuleSettings.Disable = configSettings.Module.DefinitionFile.Disable
	linterSettings.Module.ConversionsRuleSettings.Disable = configSettings.Module.Conversions.Disable
	linterSettings.Module.HelmignoreRuleSettings.Disable = configSettings.Module.Helmignore.Disable

	return linterSettings
}

func NewModule(path string, vals *chartutil.Values, globalSchema *spec.Schema, rootConfig *config.RootConfig, errorList *dmtErrors.LintRuleErrorsList) (*Module, error) {
	module, err := newModuleFromPath(path)
	if err != nil {
		return nil, err
	}

	schemas, err := ComposeValuesFromSchemas(module, globalSchema)
	if err != nil {
		return nil, err
	}

	if err = values.OverrideValues(&schemas, vals); err != nil {
		return nil, fmt.Errorf("failed to override values from file: %w", err)
	}

	objectStore := storage.NewUnstructuredObjectStore()
	err = RunRender(module, schemas, objectStore, errorList)
	if err != nil {
		return nil, err
	}
	module.objectStore = objectStore

	werfFile, err := werf.GetWerfConfig(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get werf config: %w", err)
	}
	if werfFile != "" {
		module.werfFile = werfFile
	}

	// Load module config
	cfg := &config.ModuleConfig{}
	if err := config.NewLoader(cfg, path).Load(); err != nil {
		return nil, fmt.Errorf("can not parse module config: %w", err)
	}

	cfg.LintersSettings.MergeGlobal(&rootConfig.GlobalSettings.Linters)

	module.linterConfig = remapLinterSettings(&cfg.LintersSettings, &rootConfig.GlobalSettings.Linters)

	return module, nil
}

func remapChart(ch *chart.Chart) {
	remapTemplates(ch)
	for _, dependency := range ch.Dependencies() {
		remapChart(dependency)
	}
}

//go:embed templates/_module_name.tpl
var moduleNameTemplate []byte

//go:embed templates/_module_image.tpl
var moduleImageTemplate []byte

func remapTemplates(ch *chart.Chart) {
	for _, template := range ch.Templates {
		switch template.Name {
		case "templates/_module_name.tpl":
			template.Data = moduleNameTemplate
		case "templates/_module_image.tpl":
			template.Data = moduleImageTemplate
		}
	}
}

func newModuleFromPath(path string) (*Module, error) {
	moduleYamlConfig, err := ParseModuleConfigFile(path)
	if err != nil {
		return nil, err
	}
	chartYamlConfig, err := ParseChartFile(path)
	if err != nil {
		return nil, err
	}

	var info ModuleYaml
	info.Name = GetModuleName(moduleYamlConfig, chartYamlConfig)
	if moduleYamlConfig != nil && moduleYamlConfig.Namespace != "" {
		info.Namespace = moduleYamlConfig.Namespace
	}

	if info.Namespace == "" {
		// fallback to the 'test' .namespace file
		namespace := getNamespace(path)
		if namespace == "" {
			return nil, fmt.Errorf("module %q has no namespace", info.Name)
		}
		info.Namespace = namespace
	}

	moduleChart, err := LoadModuleAsChart(info.Name, path)
	if err != nil {
		return nil, err
	}
	remapChart(moduleChart)

	resultModule := &Module{
		name:      info.Name,
		namespace: info.Namespace,
		path:      path,
		chart:     moduleChart,
	}

	return resultModule, nil
}

func getNamespace(path string) string {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(content), " \t\n")
}

func ParseModuleConfigFile(path string) (*ModuleYaml, error) {
	moduleFilename := filepath.Join(path, ModuleConfigFilename)
	yamlFile, err := os.ReadFile(moduleFilename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var moduleConfig ModuleYaml
	err = yaml.Unmarshal(yamlFile, &moduleConfig)
	if err != nil {
		return nil, err
	}

	return &moduleConfig, nil
}

func ParseChartFile(path string) (*ChartYaml, error) {
	chartFilename := filepath.Join(path, ChartConfigFilename)
	yamlFile, err := os.ReadFile(chartFilename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var chartYaml ChartYaml
	err = yaml.Unmarshal(yamlFile, &chartYaml)
	if err != nil {
		return nil, err
	}

	return &chartYaml, nil
}

func GetModuleName(moduleYamlFile *ModuleYaml, chartYamlFile *ChartYaml) string {
	if moduleYamlFile != nil && moduleYamlFile.Name != "" {
		return moduleYamlFile.Name
	}
	if chartYamlFile != nil && chartYamlFile.Name != "" {
		return chartYamlFile.Name
	}
	return ""
}
