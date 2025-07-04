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

package rules

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/mod/modfile"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	RequirementsRuleName = "requirements"
	// MinimalDeckhouseVersionForStage defines the minimum required Deckhouse version for stage usage
	MinimalDeckhouseVersionForStage = "1.68.0"
	// MinimalDeckhouseVersionForGoHooks defines the minimum required Deckhouse version for Go hooks usage
	MinimalDeckhouseVersionForGoHooks = "1.68.0"
	// MinimalDeckhouseVersionForReadinessProbes defines the minimum required Deckhouse version for readiness probes usage
	MinimalDeckhouseVersionForReadinessProbes = "1.71.0"

	// MinimalModuleSDKVersionForReadiness defines the minimum module-sdk version for readiness probes
	MinimalModuleSDKVersionForReadiness = "0.3"

	// Common patterns used in Go files
	ReadinessProbePattern = `(\w+)\.WithReadiness`
	AppRunPattern         = `\w+\.Run\(`
)

// Precompiled regex patterns for better performance
var (
	readinessProbeRegex    = regexp.MustCompile(ReadinessProbePattern)
	appRunRegex            = regexp.MustCompile(AppRunPattern)
	versionConstraintRegex = regexp.MustCompile(`([><=]=?|!=)\s*v?(\d+\.\d+\.\d+)`)
)

func NewRequirementsRule() *RequirementsRule {
	return &RequirementsRule{
		RuleMeta: pkg.RuleMeta{
			Name: RequirementsRuleName,
		},
	}
}

type RequirementsRule struct {
	pkg.RuleMeta
}

// ComponentType represents the type of component for requirements validation
type ComponentType string

const (
	ComponentDeckhouse ComponentType = "deckhouse"
	ComponentK8s       ComponentType = "kubernetes"
	ComponentModule    ComponentType = "module"
)

// ComponentRequirement defines a requirement for a specific component
type ComponentRequirement struct {
	ComponentType ComponentType
	MinVersion    string
	Description   string
}

// RequirementCheck defines a single requirement check configuration
// Detector returns true if the rule should be applied to the module
// Requirements defines the minimum versions required for this rule
// Description is the rule description
type RequirementCheck struct {
	Name         string
	Requirements []ComponentRequirement
	Description  string
	Detector     func(modulePath string, module *DeckhouseModule) bool
}

// RequirementsRegistry holds all requirement checks
type RequirementsRegistry struct {
	checks []RequirementCheck
}

// NewRequirementsRegistry creates a new registry with default checks
func NewRequirementsRegistry() *RequirementsRegistry {
	registry := &RequirementsRegistry{}

	// Stage check
	registry.RegisterCheck(RequirementCheck{
		Name: "stage",
		Requirements: []ComponentRequirement{
			{
				ComponentType: ComponentDeckhouse,
				MinVersion:    MinimalDeckhouseVersionForStage,
				Description:   "Stage usage requires minimum Deckhouse version",
			},
		},
		Description: "Stage usage requires minimum Deckhouse version",
		Detector: func(_ string, module *DeckhouseModule) bool {
			return module != nil && module.Stage != ""
		},
	})

	// Go hooks check - checks for go.mod with module-sdk dependency and app.Run calls
	registry.RegisterCheck(RequirementCheck{
		Name: "go_hooks",
		Requirements: []ComponentRequirement{
			{
				ComponentType: ComponentDeckhouse,
				MinVersion:    MinimalDeckhouseVersionForGoHooks,
				Description:   "Go hooks usage requires minimum Deckhouse version",
			},
		},
		Description: "Go hooks usage requires minimum Deckhouse version",
		Detector: func(modulePath string, _ *DeckhouseModule) bool {
			return hasGoHooks(modulePath)
		},
	})

	// Readiness probes check - checks for app.WithReadiness with module-sdk >= 0.3
	registry.RegisterCheck(RequirementCheck{
		Name: "readiness_probes",
		Requirements: []ComponentRequirement{
			{
				ComponentType: ComponentDeckhouse,
				MinVersion:    MinimalDeckhouseVersionForReadinessProbes,
				Description:   "Readiness probes usage requires minimum Deckhouse version",
			},
		},
		Description: "Readiness probes usage requires minimum Deckhouse version",
		Detector: func(modulePath string, _ *DeckhouseModule) bool {
			return hasReadinessProbes(modulePath)
		},
	})

	return registry
}

// RegisterCheck adds a new requirement check to the registry
func (r *RequirementsRegistry) RegisterCheck(check RequirementCheck) {
	r.checks = append(r.checks, check)
}

// RunAllChecks executes all registered requirement checks
func (r *RequirementsRegistry) RunAllChecks(modulePath string, module *DeckhouseModule, errorList *errors.LintRuleErrorsList) {
	for _, check := range r.checks {
		if check.Detector(modulePath, module) {
			r.validateRequirement(check, module, errorList)
		}
	}
}

// validateRequirement validates a single requirement check
func (r *RequirementsRegistry) validateRequirement(check RequirementCheck, module *DeckhouseModule, errorList *errors.LintRuleErrorsList) {
	if module == nil {
		errorList.Errorf("requirements [%s]: %s, module is not defined", check.Name, check.Description)
		return
	}

	for _, req := range check.Requirements {
		r.validateComponentRequirement(check.Name, req, module, errorList)
	}
}

// validateComponentRequirement validates a single component requirement
func (r *RequirementsRegistry) validateComponentRequirement(checkName string, req ComponentRequirement, module *DeckhouseModule, errorList *errors.LintRuleErrorsList) {
	var constraintStr string
	var constraintName string

	switch req.ComponentType {
	case ComponentDeckhouse:
		if module.Requirements == nil || module.Requirements.Deckhouse == "" {
			errorList.Errorf("requirements [%s]: %s, deckhouse version range should start no lower than %s",
				checkName, req.Description, req.MinVersion)
			return
		}
		constraintStr = module.Requirements.Deckhouse
		constraintName = "deckhouse"
	case ComponentK8s:
		if module.Requirements == nil || module.Requirements.Kubernetes == "" {
			errorList.Errorf("requirements [%s]: %s, kubernetes version constraint is required", checkName, req.Description)
			return
		}
		constraintStr = module.Requirements.Kubernetes
		constraintName = "kubernetes"
	case ComponentModule:
		// For module requirements, we would need to check specific modules
		// This is a placeholder for future implementation
		return
	default:
		errorList.Errorf("requirements [%s]: unknown component type %s", checkName, req.ComponentType)
		return
	}

	constraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		errorList.Errorf("requirements [%s]: invalid %s version constraint: %s", checkName, constraintName, constraintStr)
		return
	}

	minAllowed := findMinimalAllowedVersion(constraint)
	minimalVersion := semver.MustParse(req.MinVersion)

	if minAllowed != nil && minAllowed.LessThan(minimalVersion) {
		errorList.Errorf("requirements [%s]: %s, %s version range should start no lower than %s (currently: %s)",
			checkName, req.Description, constraintName, req.MinVersion, minAllowed.String())
	}
}

// findGoModFilesWithModuleSDK finds go.mod files that contain module-sdk dependency with version >= minVersion
func findGoModFilesWithModuleSDK(modulePath, minVersion string) []string {
	hooksDir := filepath.Join(modulePath, "hooks")

	// Check if hooks directory exists before scanning
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	goModFiles := fsutils.GetFiles(hooksDir, true, fsutils.FilterFileByNames("go.mod"))
	if len(goModFiles) == 0 {
		return nil
	}

	var validGoModDirs []string
	for _, goModFile := range goModFiles {
		if hasModuleSDKDependency(goModFile, minVersion) {
			validGoModDirs = append(validGoModDirs, filepath.Dir(goModFile))
		}
	}
	return validGoModDirs
}

// hasModuleSDKDependency checks if go.mod file contains module-sdk dependency with version >= minVersion
func hasModuleSDKDependency(goModFile, minVersion string) bool {
	goModFileContent, err := os.ReadFile(goModFile)
	if err != nil {
		return false
	}

	modFile, err := modfile.Parse(goModFile, goModFileContent, nil)
	if err != nil {
		return false
	}

	for _, req := range modFile.Require {
		if req.Mod.Path == "github.com/deckhouse/module-sdk" {
			if req.Mod.Version != "" {
				sdkVersion, err := semver.NewVersion(req.Mod.Version)
				if err == nil && !sdkVersion.LessThan(semver.MustParse(minVersion)) {
					return true
				}
			}
		}
	}
	return false
}

// findPatternInGoFiles searches for a regex pattern in Go files within the specified directories
func findPatternInGoFiles(dirs []string, pattern *regexp.Regexp) bool {
	for _, dir := range dirs {
		goFiles := fsutils.GetFiles(dir, true, fsutils.FilterFileByExtensions(".go"))
		for _, goFile := range goFiles {
			content, err := os.ReadFile(goFile)
			if err != nil {
				continue
			}
			if pattern.Match(content) {
				return true
			}
		}
	}
	return false
}

// hasReadinessProbes determines if readiness probes (app.WithReadiness) and module-sdk >= 0.3 are used
func hasReadinessProbes(modulePath string) bool {
	validGoModDirs := findGoModFilesWithModuleSDK(modulePath, MinimalModuleSDKVersionForReadiness)
	if len(validGoModDirs) == 0 {
		return false
	}

	return findPatternInGoFiles(validGoModDirs, readinessProbeRegex)
}

// hasGoHooks determines if there are go hooks with module-sdk dependency and app.Run calls
func hasGoHooks(modulePath string) bool {
	// Check that there's module-sdk dependency in go.mod files (any version)
	validGoModDirs := findGoModFilesWithModuleSDK(modulePath, "0.0")
	if len(validGoModDirs) == 0 {
		return false
	}

	// Check that there are app.Run calls only in directories with module-sdk dependency
	return findPatternInGoFiles(validGoModDirs, appRunRegex)
}

func (r *RequirementsRule) CheckRequirements(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(ModuleConfigFilename)

	moduleDescriptions, err := getDeckhouseModule(modulePath, errorList)
	if err != nil {
		return
	}

	registry := NewRequirementsRegistry()
	registry.RunAllChecks(modulePath, moduleDescriptions, errorList)
}

// findMinimalAllowedVersion finds the minimum allowed version among all >=, >, =, != in constraint
// Uses regex to extract versions and operators, returns the minimal version, or nil if only < or <= are present
func findMinimalAllowedVersion(constraint *semver.Constraints) *semver.Version {
	if constraint == nil {
		return nil
	}

	matches := versionConstraintRegex.FindAllStringSubmatch(constraint.String(), -1)
	var minVersion *semver.Version
	foundMin := false

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		op := m[1]
		verStr := m[2]
		if op == ">=" || op == ">" || op == "=" || op == "!=" {
			v, err := semver.NewVersion(verStr)
			if err == nil {
				if minVersion == nil || v.LessThan(minVersion) {
					minVersion = v
				}
				foundMin = true
			}
		}
	}

	if !foundMin {
		return nil
	}
	if minVersion != nil && constraint.Check(minVersion) {
		return minVersion
	}
	return minVersion
}

// getDeckhouseModule parse module.yaml file and return DeckhouseModule struct
func getDeckhouseModule(modulePath string, errorList *errors.LintRuleErrorsList) (*DeckhouseModule, error) {
	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if stderrors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		errorList.Errorf("Cannot stat file %q: %s", ModuleConfigFilename, err)

		return nil, err
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		errorList.Errorf("Cannot read file %q: %s", ModuleConfigFilename, err)

		return nil, err
	}

	var yml DeckhouseModule

	err = yaml.Unmarshal(yamlFile, &yml)
	if err != nil {
		errorList.Errorf("Cannot parse file %q: %s", ModuleConfigFilename, err)

		return nil, err
	}

	return &yml, nil
}
