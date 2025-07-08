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

package manager

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/go-wordwrap"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/values"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/container"
	"github.com/deckhouse/dmt/pkg/linters/hooks"
	"github.com/deckhouse/dmt/pkg/linters/images"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	no_cyrillic "github.com/deckhouse/dmt/pkg/linters/no-cyrillic"
	"github.com/deckhouse/dmt/pkg/linters/openapi"
	"github.com/deckhouse/dmt/pkg/linters/rbac"
	"github.com/deckhouse/dmt/pkg/linters/templates"
)

const (
	ChartConfigFilename = "Chart.yaml"
	ModuleYamlFilename  = "module.yaml"
	HooksDir            = "hooks"
	ImagesDir           = "images"
	OpenAPIDir          = "openapi"
)

type Linter interface {
	Run(m *module.Module)
	Name() string
}

type Manager struct {
	cfg     *config.RootConfig
	Modules []*module.Module

	errors  *errors.LintRuleErrorsList
	tracker *exclusions.ExclusionTracker
}

func NewManager(dir string, rootConfig *config.RootConfig) *Manager {
	managerLevel := pkg.Error
	m := &Manager{
		cfg: rootConfig,

		errors:  errors.NewLintRuleErrorsList().WithMaxLevel(&managerLevel),
		tracker: exclusions.NewExclusionTracker(),
	}

	paths, err := getModulePaths(dir)
	if err != nil {
		logger.ErrorF("Error getting module paths: %v", err)
		return m
	}

	vals, err := decodeValuesFile(flags.ValuesFile)
	if err != nil {
		logger.ErrorF("Failed to decode values file: %v", err)
	}

	globalValues, err := values.GetGlobalValues(getRootDirectory(dir))
	if err != nil {
		logger.ErrorF("Failed to get global values: %v", err)
		return m
	}
	errorList := m.errors.WithLinterID("manager")
	for i := range paths {
		moduleName := filepath.Base(paths[i])
		logger.DebugF("Found `%s` module", moduleName)
		if err := m.validateModule(paths[i]); err != nil {
			// linting errors are already logged
			continue
		}
		mdl, err := module.NewModule(paths[i], &vals, globalValues, errorList)
		if err != nil {
			errorList.
				WithFilePath(paths[i]).WithModule(moduleName).
				WithValue(err.Error()).
				Errorf("cannot create module `%s`", moduleName)
			continue
		}

		mdl.MergeRootConfig(rootConfig)

		m.Modules = append(m.Modules, mdl)
	}

	logger.InfoF("Found %d modules", len(m.Modules))

	return m
}

func decodeValuesFile(path string) (chartutil.Values, error) {
	if path == "" {
		return nil, nil
	}

	valuesFile, err := fsutils.ExpandDir(path)
	if err != nil {
		return nil, err
	}

	return chartutil.ReadValuesFile(valuesFile)
}

func (m *Manager) registerAllExclusions() {
	for _, mod := range m.Modules {
		cfg := mod.GetModuleConfig()
		if cfg == nil {
			continue
		}
		m.registerOpenAPIExclusions(cfg)
		m.registerNoCyrillicExclusions(cfg)
		m.registerContainerExclusions(cfg)
		m.registerTemplatesExclusions(cfg)
		m.registerRBACExclusions(cfg)
		m.registerImagesExclusions(cfg)
		m.registerHooksExclusions(cfg)
		m.registerModuleExclusions(cfg)
	}
}

func (m *Manager) registerOpenAPIExclusions(cfg *config.ModuleConfig) {
	openapiCfg := &cfg.LintersSettings.OpenAPI
	// CRD names
	crdEx := openapiCfg.OpenAPIExcludeRules.CRDNamesExcludes.Get()
	crdStr := make([]string, len(crdEx))
	for i, v := range crdEx {
		crdStr[i] = string(v)
	}
	m.tracker.RegisterExclusions("openapi", "crd-names", crdStr)
	// HA keys
	haEx := openapiCfg.OpenAPIExcludeRules.HAAbsoluteKeysExcludes.Get()
	haStr := make([]string, len(haEx))
	for i, v := range haEx {
		haStr[i] = string(v)
	}
	m.tracker.RegisterExclusions("openapi", "ha-absolute-keys", haStr)
	// Key banned names
	keyBanned := make([]string, len(openapiCfg.OpenAPIExcludeRules.KeyBannedNames))
	copy(keyBanned, openapiCfg.OpenAPIExcludeRules.KeyBannedNames)
	m.tracker.RegisterExclusions("openapi", "key-banned-names", keyBanned)
	// Enum file excludes
	enumEx := make([]string, len(openapiCfg.OpenAPIExcludeRules.EnumFileExcludes))
	copy(enumEx, openapiCfg.OpenAPIExcludeRules.EnumFileExcludes)
	m.tracker.RegisterExclusions("openapi", "enum-file-excludes", enumEx)
}

// helper for registering StringRuleExcludeList
func registerStringRuleExcludeList(tracker *exclusions.ExclusionTracker, linter, rule string, list []pkg.StringRuleExclude) {
	s := make([]string, len(list))
	for i, v := range list {
		s[i] = string(v)
	}
	tracker.RegisterExclusions(linter, rule, s)
}

// helper for registering PrefixRuleExcludeList
func registerPrefixRuleExcludeList(tracker *exclusions.ExclusionTracker, linter, rule string, list []pkg.PrefixRuleExclude) {
	s := make([]string, len(list))
	for i, v := range list {
		s[i] = string(v)
	}
	tracker.RegisterExclusions(linter, rule, s)
}

func (m *Manager) registerContainerExclusions(cfg *config.ModuleConfig) {
	containerCfg := &cfg.LintersSettings.Container
	registerContainerRule(m.tracker, "read-only-root-filesystem", containerCfg.ExcludeRules.ReadOnlyRootFilesystem.Get())
	registerContainerRule(m.tracker, "resources", containerCfg.ExcludeRules.Resources.Get())
	registerContainerRule(m.tracker, "ports", containerCfg.ExcludeRules.Ports.Get())
	registerContainerRule(m.tracker, "security-context", containerCfg.ExcludeRules.SecurityContext.Get())
	registerKindRule(m.tracker, "container", "controller-security-context", containerCfg.ExcludeRules.ControllerSecurityContext.Get())
	registerContainerRule(m.tracker, "host-network-ports", containerCfg.ExcludeRules.HostNetworkPorts.Get())
	registerContainerRule(m.tracker, "image-digest", containerCfg.ExcludeRules.ImageDigest.Get())
	registerKindRule(m.tracker, "container", "dns-policy", containerCfg.ExcludeRules.DNSPolicy.Get())
	registerContainerRule(m.tracker, "liveness-probe", containerCfg.ExcludeRules.Liveness.Get())
	registerContainerRule(m.tracker, "readiness-probe", containerCfg.ExcludeRules.Readiness.Get())
}

func registerContainerRule(tracker *exclusions.ExclusionTracker, rule string, list []pkg.ContainerRuleExclude) {
	s := make([]string, len(list))
	for i, v := range list {
		if v.Container != "" {
			s[i] = v.Kind + "/" + v.Name + "/" + v.Container
		} else {
			s[i] = v.Kind + "/" + v.Name
		}
	}
	tracker.RegisterExclusions("container", rule, s)
}

func registerKindRule(tracker *exclusions.ExclusionTracker, linter, rule string, list []pkg.KindRuleExclude) {
	s := make([]string, len(list))
	for i, v := range list {
		s[i] = v.Kind + "/" + v.Name
	}
	tracker.RegisterExclusions(linter, rule, s)
}

func (m *Manager) registerNoCyrillicExclusions(cfg *config.ModuleConfig) {
	nc := &cfg.LintersSettings.NoCyrillic
	registerStringRuleExcludeList(m.tracker, "no-cyrillic", "files", nc.NoCyrillicExcludeRules.Files.Get())
	registerPrefixRuleExcludeList(m.tracker, "no-cyrillic", "directories", nc.NoCyrillicExcludeRules.Directories.Get())
}

func (m *Manager) registerTemplatesExclusions(cfg *config.ModuleConfig) {
	templatesCfg := &cfg.LintersSettings.Templates
	// PDB absent
	pdb := templatesCfg.ExcludeRules.PDBAbsent.Get()
	pdbStr := make([]string, len(pdb))
	for i, v := range pdb {
		pdbStr[i] = v.Kind + "/" + v.Name
	}
	m.tracker.RegisterExclusions("templates", "pdb", pdbStr)

	// VPA absent
	vpa := templatesCfg.ExcludeRules.VPAAbsent.Get()
	vpaStr := make([]string, len(vpa))
	for i, v := range vpa {
		vpaStr[i] = v.Kind + "/" + v.Name
	}
	m.tracker.RegisterExclusions("templates", "vpa", vpaStr)

	// Service port
	servicePort := templatesCfg.ExcludeRules.ServicePort.Get()
	servicePortStr := make([]string, len(servicePort))
	for i, v := range servicePort {
		servicePortStr[i] = v.Name + "/" + v.Port
	}
	m.tracker.RegisterExclusions("templates", "service-port", servicePortStr)

	// Kube RBAC proxy
	kubeRbacProxy := templatesCfg.ExcludeRules.KubeRBACProxy.Get()
	kubeRbacProxyStr := make([]string, len(kubeRbacProxy))
	for i, v := range kubeRbacProxy {
		kubeRbacProxyStr[i] = string(v)
	}
	m.tracker.RegisterExclusions("templates", "kube-rbac-proxy", kubeRbacProxyStr)
}

func (m *Manager) registerRBACExclusions(cfg *config.ModuleConfig) {
	rbacCfg := &cfg.LintersSettings.Rbac
	// Binding subject
	bs := rbacCfg.ExcludeRules.BindingSubject.Get()
	bsStr := make([]string, len(bs))
	for i, v := range bs {
		bsStr[i] = string(v)
	}
	m.tracker.RegisterExclusions("rbac", "binding-subject", bsStr)

	// Placement
	placement := rbacCfg.ExcludeRules.Placement.Get()
	placementStr := make([]string, len(placement))
	for i, v := range placement {
		placementStr[i] = v.Kind + "/" + v.Name
	}
	m.tracker.RegisterExclusions("rbac", "placement", placementStr)

	// Wildcards
	wildcards := rbacCfg.ExcludeRules.Wildcards.Get()
	wildcardsStr := make([]string, len(wildcards))
	for i, v := range wildcards {
		wildcardsStr[i] = v.Kind + "/" + v.Name
	}
	m.tracker.RegisterExclusions("rbac", "wildcards", wildcardsStr)
}

func (m *Manager) registerImagesExclusions(cfg *config.ModuleConfig) {
	img := &cfg.LintersSettings.Images
	registerPrefixRuleExcludeList(m.tracker, "images", "image-file-path-prefix", img.ExcludeRules.SkipImageFilePathPrefix.Get())
	registerPrefixRuleExcludeList(m.tracker, "images", "distroless-file-path-prefix", img.ExcludeRules.SkipDistrolessFilePathPrefix.Get())
}

func (m *Manager) registerHooksExclusions(_ *config.ModuleConfig) {
	// Hooks linter doesn't have exclusion rules in current configuration
	// Register empty exclusions to track usage
	m.tracker.RegisterExclusions("hooks", "ingress", []string{})
}

func (m *Manager) registerModuleExclusions(cfg *config.ModuleConfig) {
	moduleCfg := &cfg.LintersSettings.Module
	// License files
	lic := moduleCfg.ExcludeRules.License.Files.Get()
	licStr := make([]string, len(lic))
	for i, v := range lic {
		licStr[i] = string(v)
	}
	m.tracker.RegisterExclusions("module", "license", licStr)

	// License directories
	licDirs := moduleCfg.ExcludeRules.License.Directories.Get()
	licDirsStr := make([]string, len(licDirs))
	for i, v := range licDirs {
		licDirsStr[i] = string(v)
	}
	m.tracker.RegisterExclusions("module", "license-directories", licDirsStr)

	// Conversions description
	conversionsDesc := moduleCfg.ExcludeRules.Conversions.Description.Get()
	conversionsDescStr := make([]string, len(conversionsDesc))
	for i, v := range conversionsDesc {
		conversionsDescStr[i] = string(v)
	}
	m.tracker.RegisterExclusions("module", "conversions-description", conversionsDescStr)
}

func (m *Manager) Run() {
	m.registerAllExclusions()
	wg := new(sync.WaitGroup)
	processingCh := make(chan struct{}, flags.LintersLimit)

	for _, mod := range m.Modules {
		processingCh <- struct{}{}
		wg.Add(1)

		go func(mod *module.Module) {
			defer func() {
				<-processingCh
				wg.Done()
			}()

			logger.InfoF("Run linters for `%s` module", mod.GetName())

			for _, linter := range getLintersForModule(mod.GetModuleConfig(), m.errors, m.tracker) {
				if flags.LinterName != "" && linter.Name() != flags.LinterName {
					continue
				}

				logger.DebugF("Running linter `%s` on module `%s`", linter.Name(), mod.GetName())

				linter.Run(mod)
			}
		}(mod)
	}

	wg.Wait()
}

func getLintersForModule(cfg *config.ModuleConfig, errList *errors.LintRuleErrorsList, tracker *exclusions.ExclusionTracker) []Linter {
	return []Linter{
		openapi.NewWithTracker(cfg, errList, tracker),
		no_cyrillic.NewWithTracker(cfg, errList, tracker),
		container.NewWithTracker(cfg, errList, tracker),
		templates.NewWithTracker(cfg, errList, tracker),
		images.NewWithTracker(cfg, errList, tracker),
		rbac.NewWithTracker(cfg, errList, tracker),
		hooks.NewWithTracker(cfg, errList, tracker),
		moduleLinter.NewWithTracker(cfg, errList, tracker),
	}
}

func (m *Manager) PrintResult() {
	errs := m.errors.GetErrors()

	// Print unused exclusions as warnings (always, regardless of errors)
	unusedExclusions := m.tracker.FormatUnusedExclusions()
	if unusedExclusions != "" {
		fmt.Println(color.New(color.FgHiYellow).SprintFunc()("⚠️  WARNING: "))
		fmt.Println(color.New(color.FgHiYellow).SprintFunc()(unusedExclusions))
	}

	if len(errs) == 0 {
		return
	}

	slices.SortFunc(errs, func(a, b pkg.LinterError) int {
		return cmp.Or(
			cmp.Compare(a.ModuleID, b.ModuleID),
			cmp.Compare(a.LinterID, b.LinterID),
			cmp.Compare(a.RuleID, b.RuleID),
		)
	})

	w := new(tabwriter.Writer)

	const minWidth = 5

	buf := bytes.NewBuffer([]byte{})
	w.Init(buf, minWidth, 0, 0, ' ', 0)

	for idx := range errs {
		err := errs[idx]

		msgColor := color.FgRed

		if err.Level == pkg.Warn {
			msgColor = color.FgHiYellow
		}

		metrics.IncDmtLinterErrorsCount(err.LinterID, err.RuleID, err.Level.String())

		// header
		fmt.Fprint(w, emoji.Sprintf(":monkey:"))
		fmt.Fprint(w, color.New(color.FgHiBlue).SprintFunc()("["))

		if err.RuleID != "" {
			fmt.Fprint(w, color.New(color.FgHiBlue).SprintFunc()(err.RuleID+" "))
		}

		fmt.Fprintf(w, "%s\n", color.New(color.FgHiBlue).SprintfFunc()("(#%s)]", err.LinterID))

		// body
		fmt.Fprintf(w, "\t%s\t\t%s\n", "Message:", color.New(msgColor).SprintfFunc()(prepareString(err.Text)))

		fmt.Fprintf(w, "\t%s\t\t%s\n", "Module:", err.ModuleID)

		if err.ObjectID != "" && err.ObjectID != err.ModuleID {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "Object:", err.ObjectID)
		}

		if err.ObjectValue != nil {
			value := fmt.Sprintf("%v", err.ObjectValue)

			fmt.Fprintf(w, "\t%s\t\t%s\n", "Value:", prepareString(value))
		}

		if err.FilePath != "" {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "FilePath:", strings.TrimSpace(err.FilePath))
		}

		if err.LineNumber != 0 {
			fmt.Fprintf(w, "\t%s\t\t%d\n", "LineNumber:", err.LineNumber)
		}

		fmt.Fprintln(w)

		w.Flush()
	}

	fmt.Println(buf.String())
}

func (m *Manager) HasCriticalErrors() bool {
	return m.errors.ContainsErrors()
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

// getModulePaths returns all paths with Chart.yaml
// modulesDir can be a module directory or a directory that contains helm in subdirectories.
func getModulePaths(modulesDir string) ([]string, error) {
	var chartDirs = make([]string, 0)

	// Here we find all dirs and check for Chart.yaml in them.
	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("file access '%s': %w", path, err)
		}

		// Ignore non-dirs
		if !info.IsDir() {
			return nil
		}

		// Check if first level subdirectory has a helm chart configuration file
		if isExistsOnFilesystem(path, ModuleYamlFilename) ||
			(isExistsOnFilesystem(path, ChartConfigFilename) &&
				(isExistsOnFilesystem(path, HooksDir) ||
					isExistsOnFilesystem(path, ImagesDir) ||
					isExistsOnFilesystem(path, OpenAPIDir))) {
			chartDirs = append(chartDirs, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return chartDirs, nil
}

// prepareString handle ussual string and prepare it for tablewriter
func prepareString(input string) string {
	// magic wrap const
	const wrapLen = 100

	w := &strings.Builder{}

	// split wraps for tablewrite
	split := strings.Split(wordwrap.WrapString(input, wrapLen), "\n")

	// first string must be pure for correct handling
	fmt.Fprint(w, strings.TrimSpace(split[0]))

	for i := 1; i < len(split); i++ {
		fmt.Fprintf(w, "\n\t\t\t%s", strings.TrimSpace(split[i]))
	}

	return w.String()
}

func getRootDirectory(dir string) string {
	for {
		if fsutils.IsDir(filepath.Join(dir, "global-hooks", "openapi")) &&
			fsutils.IsDir(filepath.Join(dir, "modules")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "config-values.yaml")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "values.yaml")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if dir == parent || parent == "" {
			break
		}

		dir = parent
	}

	return ""
}
