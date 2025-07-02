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
	errs "errors"
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

// RequirementCheck defines a single requirement check configuration
// Detector returns true if the rule should be applied to the module
// MinDeckhouseVersion is the minimum deckhouse version for this rule
// Description is the rule description
type RequirementCheck struct {
	Name                string
	MinDeckhouseVersion string
	Description         string
	Detector            func(modulePath string, module *DeckhouseModule) bool
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
		Name:                "stage",
		MinDeckhouseVersion: MinimalDeckhouseVersionForStage,
		Description:         "Stage usage requires minimum Deckhouse version",
		Detector: func(_ string, module *DeckhouseModule) bool {
			return module != nil && module.Stage != ""
		},
	})

	// Go hooks check - checks for go.mod with module-sdk dependency and app.Run calls
	registry.RegisterCheck(RequirementCheck{
		Name:                "go_hooks",
		MinDeckhouseVersion: MinimalDeckhouseVersionForGoHooks,
		Description:         "Go hooks usage requires minimum Deckhouse version",
		Detector: func(modulePath string, _ *DeckhouseModule) bool {
			return hasGoHooks(modulePath)
		},
	})

	// Readiness probes check - checks for app.WithReadiness with module-sdk >= 0.3
	registry.RegisterCheck(RequirementCheck{
		Name:                "readiness_probes",
		MinDeckhouseVersion: MinimalDeckhouseVersionForReadinessProbes,
		Description:         "Readiness probes usage requires minimum Deckhouse version",
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
func (*RequirementsRegistry) validateRequirement(check RequirementCheck, module *DeckhouseModule, errorList *errors.LintRuleErrorsList) {
	if module == nil || module.Requirements == nil || module.Requirements.Deckhouse == "" {
		errorList.Errorf("requirements: %s, deckhouse version range should start no lower than %s",
			check.Description, check.MinDeckhouseVersion)
		return
	}

	constraint, err := semver.NewConstraint(module.Requirements.Deckhouse)
	if err != nil {
		errorList.Errorf("invalid deckhouse version constraint: %s", module.Requirements.Deckhouse)
		return
	}

	minAllowed := findMinimalAllowedVersion(constraint)
	minimalVersion := semver.MustParse(check.MinDeckhouseVersion)

	if minAllowed != nil && minAllowed.LessThan(minimalVersion) {
		errorList.Errorf("requirements: %s, deckhouse version range should start no lower than %s (currently: %s)",
			check.Description, check.MinDeckhouseVersion, minAllowed.String())
	}
}

// findGoModFilesWithModuleSDK finds go.mod files that contain module-sdk dependency with version >= minVersion
func findGoModFilesWithModuleSDK(modulePath, minVersion string) []string {
	hooksDir := filepath.Join(modulePath, "hooks")
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

	readinessPattern := regexp.MustCompile(ReadinessProbePattern)
	return findPatternInGoFiles(validGoModDirs, readinessPattern)
}

// hasGoHooks determines if there are go hooks with module-sdk dependency and app.Run calls
func hasGoHooks(modulePath string) bool {
	// Check that there's module-sdk dependency in go.mod files (any version)
	validGoModDirs := findGoModFilesWithModuleSDK(modulePath, "0.0")
	if len(validGoModDirs) == 0 {
		return false
	}

	// Check that there are app.Run calls
	return hasAppRunCalls(modulePath)
}

// hasAppRunCalls determines if there are app.Run calls in Go files
func hasAppRunCalls(modulePath string) bool {
	hooksDir := filepath.Join(modulePath, "hooks")
	// Pattern to match any variable name followed by .Run()
	// This will match app.Run(), myApp.Run(), hookApp.Run(), etc.
	runPattern := regexp.MustCompile(AppRunPattern)
	return findPatternInGoFiles([]string{hooksDir}, runPattern)
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

// findMinimalAllowedVersion finds the minimum allowed version among all >=, >, = in the constraint string
func findMinimalAllowedVersion(constraint *semver.Constraints) *semver.Version {
	if constraint == nil {
		return nil
	}

	pattern := regexp.MustCompile(`([><=]=?)\s*v?(\d+\.\d+\.\d+)`) // finds >= 1.2.3, > 1.2.3, = 1.2.3
	matches := pattern.FindAllStringSubmatch(constraint.String(), -1)
	var minVersion *semver.Version
	for _, m := range matches {
		op := m[1]
		verStr := m[2]
		if op == ">=" || op == ">" || op == "=" {
			v, err := semver.NewVersion(verStr)
			if err == nil {
				if minVersion == nil || v.LessThan(minVersion) {
					minVersion = v
				}
			}
		}
	}
	return minVersion
}

// getDeckhouseModule parse module.yaml file and return DeckhouseModule struct
func getDeckhouseModule(modulePath string, errorList *errors.LintRuleErrorsList) (*DeckhouseModule, error) {
	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if errs.Is(err, os.ErrNotExist) {
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
