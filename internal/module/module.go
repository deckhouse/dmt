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

// Compile-time check to ensure Module implements pkg.Module interface
var _ pkg.Module = (*Module)(nil)

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

// remapLinterSettings converts configuration settings from the config package format
// to the pkg package format, mapping both rule-level configurations and exclusion rules
// across all linter domains (Container, Image, NoCyrillic, OpenAPI, Templates, RBAC, Hooks, Module).
func remapLinterSettings(configSettings *config.LintersSettings, globalConfig *global.Linters) *pkg.LintersSettings {
	linterSettings := &pkg.LintersSettings{}

	// Step 1: Configure linter-level impact settings
	mapLinterLevels(linterSettings, configSettings)

	// Step 2: Configure individual rule impact settings
	mapRuleSettings(linterSettings, configSettings, globalConfig)

	// Step 3: Map exclusion rules and additional settings
	mapExclusionRulesAndSettings(linterSettings, configSettings)

	return linterSettings
}

// mapLinterLevels sets the impact level for each linter domain
func mapLinterLevels(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	linterSettings.Container.SetLevel(configSettings.Container.Impact)
	linterSettings.Image.SetLevel(configSettings.Images.Impact)
	linterSettings.NoCyrillic.SetLevel(configSettings.NoCyrillic.Impact)
	linterSettings.OpenAPI.SetLevel(configSettings.OpenAPI.Impact)
	linterSettings.Templates.SetLevel(configSettings.Templates.Impact)
	linterSettings.RBAC.SetLevel(configSettings.Rbac.Impact)
	linterSettings.Hooks.SetLevel(configSettings.Hooks.Impact)
	linterSettings.Module.SetLevel(configSettings.Module.Impact)
	linterSettings.Documentation.SetLevel(configSettings.Documentation.Impact)
}

// mapRuleSettings configures individual rules with their specific impact levels
func mapRuleSettings(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings, globalConfig *global.Linters) {
	// Container rules (uses global rule config + local fallback)
	mapContainerRules(linterSettings, configSettings, globalConfig)

	// Image rules (uses global rule config + local fallback)
	mapImageRules(linterSettings, configSettings, globalConfig)

	// Other linter rules (use local linter-level impact)
	mapSimpleLinterRules(linterSettings, configSettings)
}

// mapContainerRules configures Container linter rules
func mapContainerRules(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings, globalConfig *global.Linters) {
	linterSettings.Container.Rules.RecommendedLabelsRule.SetLevel(
		globalConfig.Container.RecommendedLabelsRule.Impact,
		configSettings.Container.Impact,
	)
}

// mapImageRules configures Image linter rules
func mapImageRules(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings, globalConfig *global.Linters) {
	rules := &linterSettings.Image.Rules
	globalRules := &globalConfig.Images.Rules
	fallbackImpact := configSettings.Images.Impact

	rules.DistrolessRule.SetLevel(globalRules.DistrolessRule.Impact, fallbackImpact)
	rules.ImageRule.SetLevel(globalRules.ImageRule.Impact, fallbackImpact)
	rules.PatchesRule.SetLevel(globalRules.PatchesRule.Impact, fallbackImpact)
	rules.WerfRule.SetLevel(globalRules.WerfRule.Impact, fallbackImpact)
}

// mapSimpleLinterRules configures rules that use linter-level impact without global overrides
func mapSimpleLinterRules(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	// NoCyrillic rules
	linterSettings.NoCyrillic.Rules.NoCyrillicRule.SetLevel("", configSettings.NoCyrillic.Impact)

	// OpenAPI rules
	openAPIImpact := configSettings.OpenAPI.Impact
	linterSettings.OpenAPI.Rules.EnumRule.SetLevel("", openAPIImpact)
	linterSettings.OpenAPI.Rules.HARule.SetLevel("", openAPIImpact)
	linterSettings.OpenAPI.Rules.CRDsRule.SetLevel("", openAPIImpact)
	linterSettings.OpenAPI.Rules.KeysRule.SetLevel("", openAPIImpact)

	// Templates rules
	templatesImpact := configSettings.Templates.Impact
	templates := &linterSettings.Templates.Rules
	templates.VPARule.SetLevel("", templatesImpact)
	templates.PDBRule.SetLevel("", templatesImpact)
	templates.IngressRule.SetLevel("", templatesImpact)
	templates.PrometheusRule.SetLevel("", templatesImpact)
	templates.GrafanaRule.SetLevel("", templatesImpact)
	templates.KubeRBACProxyRule.SetLevel("", templatesImpact)
	templates.ServicePortRule.SetLevel("", templatesImpact)
	templates.ClusterDomainRule.SetLevel("", templatesImpact)

	// RBAC rules
	rbacImpact := configSettings.Rbac.Impact
	rbac := &linterSettings.RBAC.Rules
	rbac.UserAuthRule.SetLevel("", rbacImpact)
	rbac.BindingRule.SetLevel("", rbacImpact)
	rbac.PlacementRule.SetLevel("", rbacImpact)
	rbac.WildcardsRule.SetLevel("", rbacImpact)

	// Hooks rules
	linterSettings.Hooks.Rules.HooksRule.SetLevel("", configSettings.Hooks.Impact)

	// Module rules
	moduleImpact := configSettings.Module.Impact
	module := &linterSettings.Module.Rules
	module.DefinitionFileRule.SetLevel("", moduleImpact)
	module.OSSRule.SetLevel("", moduleImpact)
	module.ConversionRule.SetLevel("", moduleImpact)
	module.HelmignoreRule.SetLevel("", moduleImpact)
	module.LicenseRule.SetLevel("", moduleImpact)
	module.RequarementsRule.SetLevel("", moduleImpact)

	// Documentation rules
	documentationImpact := configSettings.Documentation.Impact
	documentation := &linterSettings.Documentation.Rules
	documentation.BilingualRule.SetLevel("", documentationImpact)
	documentation.CyrillicInEnglishRule.SetLevel("", documentationImpact)
	documentation.ReadmeRule.SetLevel("", documentationImpact)
}

// mapExclusionRulesAndSettings maps exclusion rules and additional linter settings
func mapExclusionRulesAndSettings(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	mapContainerExclusions(linterSettings, configSettings)
	mapImageExclusionsAndSettings(linterSettings, configSettings)
	mapNoCyrillicExclusions(linterSettings, configSettings)
	mapOpenAPIExclusions(linterSettings, configSettings)
	mapTemplatesExclusionsAndSettings(linterSettings, configSettings)
	mapRBACExclusions(linterSettings, configSettings)
	mapHooksSettings(linterSettings, configSettings)
	mapModuleExclusionsAndSettings(linterSettings, configSettings)
	// no excluded rules - mapDocumentationExclusionsAndSettings(linterSettings, configSettings)
}

// mapContainerExclusions maps Container linter exclusion rules
func mapContainerExclusions(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	excludes := &linterSettings.Container.ExcludeRules
	configExcludes := &configSettings.Container.ExcludeRules

	excludes.ControllerSecurityContext = configExcludes.ControllerSecurityContext.Get()
	excludes.DNSPolicy = configExcludes.DNSPolicy.Get()
	excludes.HostNetworkPorts = configExcludes.HostNetworkPorts.Get()
	excludes.Ports = configExcludes.Ports.Get()
	excludes.ReadOnlyRootFilesystem = configExcludes.ReadOnlyRootFilesystem.Get()
	excludes.ImageDigest = configExcludes.ImageDigest.Get()
	excludes.Resources = configExcludes.Resources.Get()
	excludes.SecurityContext = configExcludes.SecurityContext.Get()
	excludes.Liveness = configExcludes.Liveness.Get()
	excludes.Readiness = configExcludes.Readiness.Get()
	excludes.Description = pkg.StringRuleExcludeList(configExcludes.Description)
}

// mapImageExclusionsAndSettings maps Image linter exclusions and additional settings
func mapImageExclusionsAndSettings(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	// Exclusion rules
	excludes := &linterSettings.Image.ExcludeRules
	configExcludes := &configSettings.Images.ExcludeRules
	excludes.SkipImageFilePathPrefix = pkg.PrefixRuleExcludeList(configExcludes.SkipImageFilePathPrefix)
	excludes.SkipDistrolessFilePathPrefix = pkg.PrefixRuleExcludeList(configExcludes.SkipDistrolessFilePathPrefix)

	// Additional settings
	linterSettings.Image.Patches.Disable = configSettings.Images.Patches.Disable
	linterSettings.Image.Werf.Disable = configSettings.Images.Werf.Disable
}

// mapNoCyrillicExclusions maps NoCyrillic linter exclusion rules
func mapNoCyrillicExclusions(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	excludes := &linterSettings.NoCyrillic.ExcludeRules
	configExcludes := &configSettings.NoCyrillic.NoCyrillicExcludeRules

	excludes.Files = pkg.StringRuleExcludeList(configExcludes.Files)
	excludes.Directories = pkg.PrefixRuleExcludeList(configExcludes.Directories)
}

// mapOpenAPIExclusions maps OpenAPI linter exclusion rules
func mapOpenAPIExclusions(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	excludes := &linterSettings.OpenAPI.ExcludeRules
	configExcludes := &configSettings.OpenAPI.OpenAPIExcludeRules

	excludes.KeyBannedNames = configExcludes.KeyBannedNames
	excludes.EnumFileExcludes = configExcludes.EnumFileExcludes
	excludes.HAAbsoluteKeysExcludes = pkg.StringRuleExcludeList(configExcludes.HAAbsoluteKeysExcludes)
	excludes.CRDNamesExcludes = pkg.StringRuleExcludeList(configExcludes.CRDNamesExcludes)
}

// mapTemplatesExclusionsAndSettings maps Templates linter exclusions and settings
func mapTemplatesExclusionsAndSettings(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	// Exclusion rules
	excludes := &linterSettings.Templates.ExcludeRules
	configExcludes := &configSettings.Templates.ExcludeRules
	excludes.VPAAbsent = configExcludes.VPAAbsent.Get()
	excludes.PDBAbsent = configExcludes.PDBAbsent.Get()
	excludes.ServicePort = configExcludes.ServicePort.Get()
	excludes.KubeRBACProxy = pkg.StringRuleExcludeList(configExcludes.KubeRBACProxy)
	excludes.Ingress = configExcludes.Ingress.Get()

	// Additional settings
	linterSettings.Templates.PrometheusRuleSettings.Disable = configSettings.Templates.PrometheusRules.Disable
	linterSettings.Templates.GrafanaDashboardsSettings.Disable = configSettings.Templates.GrafanaDashboards.Disable
}

// mapRBACExclusions maps RBAC linter exclusion rules
func mapRBACExclusions(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	excludes := &linterSettings.RBAC.ExcludeRules
	configExcludes := &configSettings.Rbac.ExcludeRules

	excludes.BindingSubject = pkg.StringRuleExcludeList(configExcludes.BindingSubject)
	excludes.Placement = configExcludes.Placement.Get()
	excludes.Wildcards = configExcludes.Wildcards.Get()
}

// mapHooksSettings maps Hooks linter settings
func mapHooksSettings(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	linterSettings.Hooks.IngressRuleSettings.Disable = configSettings.Hooks.Ingress.Disable
}

// mapModuleExclusionsAndSettings maps Module linter exclusions and settings
func mapModuleExclusionsAndSettings(linterSettings *pkg.LintersSettings, configSettings *config.LintersSettings) {
	// Exclusion rules
	excludes := &linterSettings.Module.ExcludeRules
	configExcludes := &configSettings.Module.ExcludeRules
	excludes.License.Files = pkg.StringRuleExcludeList(configExcludes.License.Files)
	excludes.License.Directories = pkg.PrefixRuleExcludeList(configExcludes.License.Directories)

	// Additional settings
	linterSettings.Module.OSSRuleSettings.Disable = configSettings.Module.OSS.Disable
	linterSettings.Module.DefinitionFileRuleSettings.Disable = configSettings.Module.DefinitionFile.Disable
	linterSettings.Module.ConversionsRuleSettings.Disable = configSettings.Module.Conversions.Disable
	linterSettings.Module.HelmignoreRuleSettings.Disable = configSettings.Module.Helmignore.Disable
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
