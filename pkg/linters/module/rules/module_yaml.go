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
	"slices"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/ini.v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	DefinitionFileRuleName = "definition-file"
)

// Valid edition values
var ValidEditions = []string{
	"ce",
	"fe",
	"ee",
	"se",
	"se-plus",
	"be",
	"_default",
}

// Valid bundle values
var ValidBundles = []string{
	"Minimal",
	"Managed",
	"Default",
}

func NewDefinitionFileRule(disable bool) *DefinitionFileRule {
	return &DefinitionFileRule{
		RuleMeta: pkg.RuleMeta{
			Name: DefinitionFileRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
	}
}

type DefinitionFileRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

const (
	ModuleConfigFilename = "module.yaml"
)

type DeckhouseModule struct {
	Name          string               `json:"name"`
	Critical      bool                 `json:"critical,omitempty"`
	Namespace     string               `json:"namespace"`
	Weight        uint32               `json:"weight,omitempty"`
	Tags          []string             `json:"tags"`
	Subsystems    []string             `json:"subsystems,omitempty"`
	Stage         string               `json:"stage"`
	Description   string               `json:"description,omitempty"`
	Descriptions  ModuleDescriptions   `json:"descriptions,omitempty"`
	Requirements  *ModuleRequirements  `json:"requirements,omitempty"`
	Accessibility *ModuleAccessibility `json:"accessibility,omitempty"`
	Update        *ModuleUpdate        `json:"update,omitempty"`
}

type ModuleDescriptions struct {
	English string `json:"en,omitempty"`
	Russian string `json:"ru,omitempty"`
}

type ModuleRequirements struct {
	ModulePlatformRequirements `json:",inline"`
	ParentModules              map[string]string `json:"modules,omitempty"`
}

type ModulePlatformRequirements struct {
	Deckhouse    string `json:"deckhouse,omitempty"`
	Kubernetes   string `json:"kubernetes,omitempty"`
	Bootstrapped bool   `json:"bootstrapped,omitempty"`
}

type ModuleAccessibility struct {
	Editions map[string]ModuleEdition `json:"editions"`
}

type ModuleEdition struct {
	Available        bool     `json:"available"`
	EnabledInBundles []string `json:"enabledInBundles"`
}

type ModuleUpdate struct {
	Versions []ModuleUpdateVersion `json:"versions,omitempty"`
}

type ModuleUpdateVersion struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func getModuleNameFromRepository(dir string) string {
	configFile := getGitConfigFile(dir)
	if configFile == "" {
		return ""
	}

	cfg, err := ini.Load(configFile)
	if err != nil {
		logger.ErrorF("Failed to load config file: %v", err)
		return ""
	}

	sec, err := cfg.GetSection("remote \"origin\"")
	if err != nil {
		logger.ErrorF("Failed to get remote \"origin\": %v", err)
		return ""
	}

	repositoryURL := sec.Key("url").String()
	return convertURLToModuleName(repositoryURL)
}

func getGitConfigFile(dir string) string {
	for {
		if fsutils.IsDir(filepath.Join(dir, ".git")) &&
			fsutils.IsFile(filepath.Join(dir, ".git", "config")) {
			return filepath.Join(dir, ".git", "config")
		}
		parent := filepath.Dir(dir)
		if dir == parent || parent == "" {
			break
		}

		dir = parent
	}

	return ""
}

// convertURLToModuleName converts a repository URL to a module name.
// It handles both SSH and HTTPS formats.
// Examples:
// git@github.com:deckhouse/dmt.git
// https://github.com/deckhouse/dmt
// It returns the last part of the URL as the module name.
// For example, for the URL "git@github.com:deckhouse/dmt.git", it will return "dmt".
func convertURLToModuleName(repoURL string) string {
	// Remove the protocol part if it exists
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "git@")

	// Remove the ".git" suffix if it exists
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Split by '/' and return the last part
	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[len(parts)-1]
}

func (r *DefinitionFileRule) CheckDefinitionFile(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(ModuleConfigFilename)

	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	_, err := os.Stat(filepath.Join(modulePath, ModuleConfigFilename))
	if errs.Is(err, os.ErrNotExist) {
		return
	}

	if err != nil {
		errorList.Errorf("Cannot stat file %q: %s", ModuleConfigFilename, err)

		return
	}

	yamlFile, err := os.ReadFile(filepath.Join(modulePath, ModuleConfigFilename))
	if err != nil {
		errorList.Errorf("Cannot read file %q: %s", ModuleConfigFilename, err)

		return
	}

	var yml DeckhouseModule

	err = yaml.Unmarshal(yamlFile, &yml)
	if err != nil {
		errorList.Errorf("Cannot parse file %q: %s", ModuleConfigFilename, err)

		return
	}

	if yml.Name == "" {
		errorList.Error("Field 'name' is required")
	}

	const maxNameLength = 64
	if len(yml.Name) > maxNameLength {
		errorList.Error("Field 'name' must not exceed 64 characters")
	}

	stages := []string{
		"Experimental",
		"Preview",
		"General Availability",
		"Deprecated",
	}

	if yml.Stage == "" {
		errorList.Error("Field 'stage' is required")
	}

	if yml.Stage != "" && !slices.Contains(stages, yml.Stage) {
		errorList.Errorf("Field 'stage' is not one of the following values: %q", strings.Join(stages, ", "))
	}

	if yml.Requirements != nil {
		yml.Requirements.validateRequirements(errorList)
	}

	if yml.Accessibility != nil {
		yml.Accessibility.validateAccessibility(errorList)
	}

	if yml.Update != nil {
		yml.Update.validateUpdate(errorList)
	}

	// TODO: refactor this
	maxLevel := ptr.To(pkg.Error)
	moduleNameFromRepo := getModuleNameFromRepository(modulePath)
	for _, repo := range pkg.IgnoreDeckhouseReposList {
		if moduleNameFromRepo == repo {
			maxLevel = ptr.To(pkg.Warn)
			break
		}
	}

	// ru description is not required
	if yml.Descriptions.English == "" {
		errorList.WithMaxLevel(maxLevel).Error("Module `descriptions.en` field is required")
	}

	if yml.Description != "" {
		errorList.WithMaxLevel(maxLevel).Error("Field 'description' is deprecated, use 'descriptions.en' instead")
	}

	if yml.Critical && yml.Weight == 0 {
		errorList.Error("Field 'weight' must not be zero for critical modules")
	}
}

func (m ModuleRequirements) validateRequirements(errorList *errors.LintRuleErrorsList) {
	if m.Deckhouse != "" {
		if _, err := semver.NewConstraint(m.Deckhouse); err != nil {
			errorList.Errorf("Invalid Deckhouse version requirement: %s", err)
		}
	}

	if m.Kubernetes != "" {
		if _, err := semver.NewConstraint(m.Kubernetes); err != nil {
			errorList.Errorf("Invalid Kubernetes version requirement: %s", err)
		}
	}

	for parentModuleName, parentModuleVersion := range m.ParentModules {
		// Parse constraint by removing the !optional flag first
		constraintStr := strings.TrimSpace(strings.ReplaceAll(parentModuleVersion, "!optional", ""))

		if _, err := semver.NewConstraint(constraintStr); err != nil {
			errorList.Errorf("Invalid module %q version requirement: %s", parentModuleName, err)
		}
	}
}

func (a *ModuleAccessibility) validateAccessibility(errorList *errors.LintRuleErrorsList) {
	if len(a.Editions) == 0 {
		errorList.Error("Field 'accessibility.editions' is required when 'accessibility' is specified")
		return
	}

	for editionName, edition := range a.Editions {
		// Validate edition name
		if !slices.Contains(ValidEditions, editionName) {
			errorList.Errorf("Invalid edition name %q. Must be one of: %s", editionName, strings.Join(ValidEditions, ", "))
		}

		// Validate enabledInBundles values
		if len(edition.EnabledInBundles) > 0 {
			for _, bundle := range edition.EnabledInBundles {
				if !slices.Contains(ValidBundles, bundle) {
					errorList.Errorf("Invalid bundle %q for edition %q. Must be one of: %s", bundle, editionName, strings.Join(ValidBundles, ", "))
				}
			}
		}
	}
}

func (u *ModuleUpdate) validateUpdate(errorList *errors.LintRuleErrorsList) {
	if len(u.Versions) == 0 {
		return
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+$`)

	for i, version := range u.Versions {
		if version.From == "" {
			errorList.Errorf("Version entry at index %d: field 'from' is required", i)
			continue
		}

		if version.To == "" {
			errorList.Errorf("Version entry at index %d: field 'to' is required", i)
			continue
		}

		if !versionPattern.MatchString(version.From) {
			errorList.Errorf("Version entry at index %d: 'from' version '%s' must be in major.minor format (patch versions not allowed)", i, version.From)
			continue
		}

		if !versionPattern.MatchString(version.To) {
			errorList.Errorf("Version entry at index %d: 'to' version '%s' must be in major.minor format (patch versions not allowed)", i, version.To)
			continue
		}

		fromVer, err := semver.NewVersion(version.From)
		if err != nil {
			errorList.Errorf("Version entry at index %d: invalid 'from' version '%s': %s", i, version.From, err)
			continue
		}

		toVer, err := semver.NewVersion(version.To)
		if err != nil {
			errorList.Errorf("Version entry at index %d: invalid 'to' version '%s': %s", i, version.To, err)
			continue
		}

		if !toVer.GreaterThan(fromVer) {
			errorList.Errorf("Version entry at index %d: 'to' version '%s' must be greater than 'from' version '%s'", i, version.To, version.From)
		}
	}

	u.validateUpdateSorting(errorList)
	u.validateUpdateDuplicates(errorList)
}

func (u *ModuleUpdate) validateUpdateSorting(errorList *errors.LintRuleErrorsList) {
	if len(u.Versions) <= 1 {
		return
	}

	sortedVersions := make([]ModuleUpdateVersion, len(u.Versions))
	copy(sortedVersions, u.Versions)

	sort.Slice(sortedVersions, func(i, j int) bool {
		fromI, errI := semver.NewVersion(sortedVersions[i].From)
		fromJ, errJ := semver.NewVersion(sortedVersions[j].From)

		if errI != nil || errJ != nil {
			return sortedVersions[i].From < sortedVersions[j].From
		}

		if fromI.Equal(fromJ) {
			toI, errI := semver.NewVersion(sortedVersions[i].To)
			toJ, errJ := semver.NewVersion(sortedVersions[j].To)

			if errI != nil || errJ != nil {
				return sortedVersions[i].To < sortedVersions[j].To
			}

			return toI.LessThan(toJ)
		}

		return fromI.LessThan(fromJ)
	})

	for i, original := range u.Versions {
		if original.From != sortedVersions[i].From || original.To != sortedVersions[i].To {
			errorList.Error("Update versions must be sorted by 'from' version ascending, then by 'to' version ascending")
			break
		}
	}
}

func (u *ModuleUpdate) validateUpdateDuplicates(errorList *errors.LintRuleErrorsList) {
	toMap := make(map[string][]string)

	for _, version := range u.Versions {
		if version.From == "" || version.To == "" {
			continue
		}
		toMap[version.To] = append(toMap[version.To], version.From)
	}

	for to, froms := range toMap {
		if len(froms) > 1 {
			sort.Strings(froms)
			errorList.Errorf("Duplicate 'to' version '%s' with different 'from' versions: %s. Use the earliest 'from' version instead", to, strings.Join(froms, ", "))
		}
	}
}
