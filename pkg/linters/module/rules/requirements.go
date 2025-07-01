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
	// MinimalDeckhouseVersionForReadinessProbes defines the minimum required Deckhouse version for readiness probes usage
	MinimalDeckhouseVersionForReadinessProbes = "1.71.0"
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
// ErrorMessage is the error message template
// Description is the rule description
type RequirementCheck struct {
	Name                string
	MinDeckhouseVersion string
	Description         string
	Detector            func(modulePath string, module *DeckhouseModule) bool
	ErrorMessage        string
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
		ErrorMessage: "stage should be used with requirements: deckhouse >= %s",
	})

	// Go hooks check (only if there's no module-sdk >= 0.3)
	registry.RegisterCheck(RequirementCheck{
		Name:                "go_hooks",
		MinDeckhouseVersion: MinimalDeckhouseVersionForStage,
		Description:         "Go hooks usage requires minimum Deckhouse version",
		Detector: func(modulePath string, _ *DeckhouseModule) bool {
			hooksDir := filepath.Join(modulePath, "hooks")
			goModFiles := fsutils.GetFiles(hooksDir, true, fsutils.FilterFileByNames("go.mod"))
			if len(goModFiles) == 0 {
				return false
			}
			// Check that there's no module-sdk >= 0.3
			return !hasModuleSDK03(modulePath) && !hasReadinessProbes(modulePath)
		},
		ErrorMessage: "requirements: for using go_hook, deckhouse version constraint must be specified (minimum: %s)",
	})

	// Readiness probes check (by presence of app.WithReadiness and module-sdk >= 0.3)
	registry.RegisterCheck(RequirementCheck{
		Name:                "readiness_probes",
		MinDeckhouseVersion: MinimalDeckhouseVersionForReadinessProbes,
		Description:         "Readiness probes usage requires minimum Deckhouse version",
		Detector: func(modulePath string, _ *DeckhouseModule) bool {
			return hasReadinessProbes(modulePath)
		},
		ErrorMessage: "requirements: for using readiness probes, deckhouse version constraint must be specified (minimum: %s)",
	})

	// module-sdk >= 0.3 check (without app.WithReadiness)
	registry.RegisterCheck(RequirementCheck{
		Name:                "module_sdk_0_3",
		MinDeckhouseVersion: MinimalDeckhouseVersionForReadinessProbes,
		Description:         "module-sdk >= 0.3 requires minimum Deckhouse version",
		Detector: func(modulePath string, _ *DeckhouseModule) bool {
			return hasModuleSDK03(modulePath)
		},
		ErrorMessage: "requirements: for using module-sdk >= 0.3, deckhouse version constraint must be specified (minimum: %s)",
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
	if module == nil || module.Requirements == nil || module.Requirements.Deckhouse == "" {
		errorList.Errorf(check.ErrorMessage, check.MinDeckhouseVersion)
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

// hasReadinessProbes determines if readiness probes (app.WithReadiness) and module-sdk >= 0.3 are used
func hasReadinessProbes(modulePath string) bool {
	goModFiles := fsutils.GetFiles(filepath.Join(modulePath, "hooks"), true, fsutils.FilterFileByNames("go.mod"))
	if len(goModFiles) == 0 {
		return false
	}
	var validGoModDirs []string
	for _, goModFile := range goModFiles {
		goModFileContent, err := os.ReadFile(goModFile)
		if err != nil {
			continue
		}
		modFile, err := modfile.Parse(goModFile, goModFileContent, nil)
		if err != nil {
			continue
		}
		for _, req := range modFile.Require {
			if req.Mod.Path == "github.com/deckhouse/module-sdk" {
				if req.Mod.Version != "" {
					sdkVersion, err := semver.NewVersion(req.Mod.Version)
					if err == nil && !sdkVersion.LessThan(semver.MustParse("0.3")) {
						validGoModDirs = append(validGoModDirs, filepath.Dir(goModFile))
						break
					}
				}
			}
		}
	}
	if len(validGoModDirs) == 0 {
		return false
	}
	for _, goModDir := range validGoModDirs {
		goFiles := fsutils.GetFiles(goModDir, true, fsutils.FilterFileByExtensions(".go"))
		for _, goFile := range goFiles {
			content, err := os.ReadFile(goFile)
			if err != nil {
				continue
			}
			readinessPattern := regexp.MustCompile(`(\w+)\.WithReadiness`)
			if readinessPattern.Match(content) {
				return true
			}
		}
	}
	return false
}

// hasModuleSDK03 determines if there's module-sdk >= 0.3 without app.WithReadiness
func hasModuleSDK03(modulePath string) bool {
	goModFiles := fsutils.GetFiles(filepath.Join(modulePath, "hooks"), true, fsutils.FilterFileByNames("go.mod"))
	if len(goModFiles) == 0 {
		return false
	}
	for _, goModFile := range goModFiles {
		goModFileContent, err := os.ReadFile(goModFile)
		if err != nil {
			continue
		}
		modFile, err := modfile.Parse(goModFile, goModFileContent, nil)
		if err != nil {
			continue
		}
		for _, req := range modFile.Require {
			if req.Mod.Path == "github.com/deckhouse/module-sdk" {
				if req.Mod.Version != "" {
					sdkVersion, err := semver.NewVersion(req.Mod.Version)
					if err == nil && !sdkVersion.LessThan(semver.MustParse("0.3")) {
						// Check that there's no app.WithReadiness
						goFiles := fsutils.GetFiles(filepath.Dir(goModFile), true, fsutils.FilterFileByExtensions(".go"))
						readinessPattern := regexp.MustCompile(`(\w+)\.WithReadiness`)
						found := false
						for _, goFile := range goFiles {
							content, err := os.ReadFile(goFile)
							if err != nil {
								continue
							}
							if readinessPattern.Match(content) {
								found = true
								break
							}
						}
						if !found {
							return true
						}
					}
				}
			}
		}
	}
	return false
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
