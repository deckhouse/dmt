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
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/mod/modfile"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	DeckhouseVersionRequirementRuleName = "deckhouse-version-requirement"
	MinModuleSDKVersion                 = "1.3.0"
	MinDeckhouseVersion                 = "1.71.0"
	ModuleConfigFilename                = "module.yaml"
)

type DeckhouseModule struct {
	Requirements *ModuleRequirements `json:"requirements,omitempty"`
}

type ModuleRequirements struct {
	Deckhouse string `json:"deckhouse,omitempty"`
}

func NewDeckhouseVersionRequirementRule() *DeckhouseVersionRequirementRule {
	return &DeckhouseVersionRequirementRule{
		RuleMeta: pkg.RuleMeta{
			Name: DeckhouseVersionRequirementRuleName,
		},
	}
}

type DeckhouseVersionRequirementRule struct {
	pkg.RuleMeta
}

func (r *DeckhouseVersionRequirementRule) CheckDeckhouseVersionRequirement(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	path := object.GetPath()
	if path == "" {
		path = object.AbsPath
	}

	// First check if Module-SDK version >= 1.3 in any go.mod files
	// We need to check this before looking for module.yaml
	hasRequiredSDKVersion, err := checkModuleSDKVersionFromPath(path)
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Cannot check Module-SDK version: %s", err)
		return
	}

	if !hasRequiredSDKVersion {
		return
	}

	modulePath := getModulePath(path)
	if modulePath == "" {
		errorList.WithObjectID(object.Identity()).
			Errorf("Module-SDK version >= %s requires Deckhouse version >= %s, but module.yaml not found", MinModuleSDKVersion, MinDeckhouseVersion)
		return
	}

	// Check Deckhouse version requirement
	module, err := getDeckhouseModule(modulePath)
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Cannot parse module.yaml: %s", err)
		return
	}

	if module == nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Module-SDK version >= %s requires Deckhouse version >= %s, but module.yaml not found", MinModuleSDKVersion, MinDeckhouseVersion)
		return
	}

	if module.Requirements == nil || module.Requirements.Deckhouse == "" {
		errorList.WithObjectID(object.Identity()).
			Errorf("Module-SDK version >= %s requires Deckhouse version >= %s, but requirements.deckhouse is not specified", MinModuleSDKVersion, MinDeckhouseVersion)
		return
	}

	// Parse and validate Deckhouse version constraint
	constraint, err := semver.NewConstraint(module.Requirements.Deckhouse)
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Invalid Deckhouse version constraint '%s': %s", module.Requirements.Deckhouse, err)
		return
	}

	minRequiredVersion, err := semver.NewVersion(MinDeckhouseVersion)
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Invalid minimum Deckhouse version format: %s", err)
		return
	}

	// Check if the constraint allows the minimum required version
	if !constraint.Check(minRequiredVersion) {
		minAllowed := findMinimalAllowedVersion(constraint)
		if minAllowed != nil {
			errorList.WithObjectID(object.Identity()).
				Errorf("Module-SDK version >= %s requires Deckhouse version >= %s, but current constraint '%s' allows minimum %s",
					MinModuleSDKVersion, MinDeckhouseVersion, module.Requirements.Deckhouse, minAllowed.String())
		} else {
			errorList.WithObjectID(object.Identity()).
				Errorf("Module-SDK version >= %s requires Deckhouse version >= %s, but current constraint '%s' does not allow this version",
					MinModuleSDKVersion, MinDeckhouseVersion, module.Requirements.Deckhouse)
		}
	}
}

func getModulePath(objectPath string) string {
	dir := filepath.Dir(objectPath)
	for {
		if dir == "/" || dir == "." {
			return ""
		}

		moduleYamlPath := filepath.Join(dir, ModuleConfigFilename)
		if _, err := os.Stat(moduleYamlPath); err == nil {
			return dir
		}

		dir = filepath.Dir(dir)
	}
}

// checkModuleSDKVersionFromPath checks if Module-SDK version >= 1.3 in any go.mod files from the given path
func checkModuleSDKVersionFromPath(objectPath string) (bool, error) {
	dir := filepath.Dir(objectPath)
	for dir != "/" && dir != "." {
		hooksDir := filepath.Join(dir, "hooks")
		if _, err := os.Stat(hooksDir); err == nil {
			if hasSDK, err := checkModuleSDKVersionInDirectory(hooksDir); err != nil {
				return false, err
			} else if hasSDK {
				return true, nil
			}
		}

		dir = filepath.Dir(dir)
	}

	return false, nil
}

// checkModuleSDKVersionInDirectory checks if Module-SDK version >= 1.3 in a specific directory
func checkModuleSDKVersionInDirectory(dir string) (bool, error) {
	goModFiles := findGoModFiles(dir)
	if len(goModFiles) == 0 {
		return false, nil
	}

	minSDKVersion, err := semver.NewVersion(MinModuleSDKVersion)
	if err != nil {
		return false, err
	}

	for _, goModFile := range goModFiles {
		if hasModuleSDKDependency(goModFile, minSDKVersion) {
			return true, nil
		}
	}

	return false, nil
}

func findGoModFiles(dir string) []string {
	var goModFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "go.mod" {
			goModFiles = append(goModFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil
	}

	return goModFiles
}

// hasModuleSDKDependency checks if go.mod file contains module-sdk dependency with version >= minVersion
func hasModuleSDKDependency(goModFile string, minVersion *semver.Version) bool {
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
				if err != nil {
					continue
				}
				if !sdkVersion.LessThan(minVersion) {
					return true
				}
			}
		}
	}

	return false
}

// getDeckhouseModule parses module.yaml file
func getDeckhouseModule(modulePath string) (*DeckhouseModule, error) {
	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		return nil, err
	}

	var module DeckhouseModule
	err = yaml.Unmarshal(yamlFile, &module)
	if err != nil {
		return nil, err
	}

	return &module, nil
}

// findMinimalAllowedVersion finds the minimal version allowed by the constraint
func findMinimalAllowedVersion(constraint *semver.Constraints) *semver.Version {
	if constraint == nil {
		return nil
	}
	constraintStr := constraint.String()
	if strings.Contains(constraintStr, ">=") {
		parts := strings.Split(constraintStr, ">=")
		if len(parts) > 1 {
			versionStr := strings.TrimSpace(strings.Split(parts[1], ",")[0])
			versionStr = strings.TrimSpace(strings.Split(versionStr, " ")[0])
			if version, err := semver.NewVersion(versionStr); err == nil {
				return version
			}
		}
	}

	return nil
}
