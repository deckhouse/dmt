/*
Copyright 2026 Flant JSC

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

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	MountPointsRuleName = "mount-points"
)

type mountPointsFile struct {
	Dirs  []string `yaml:"dirs"`
	Files []string `yaml:"files"`
}

// builtinExcludedPaths are system paths that are always available on Linux hosts
// and should not be required in mount-points.yaml.
var builtinExcludedPaths = map[string]bool{
	"/sys":  true,
	"/dev":  true,
	"/proc": true,
}

func NewMountPointsRule(excludeRules []pkg.StringRuleExclude) *MountPointsRule {
	return &MountPointsRule{
		RuleMeta: pkg.RuleMeta{
			Name: MountPointsRuleName,
		},
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
	}
}

type MountPointsRule struct {
	pkg.RuleMeta
	pkg.StringRule
}

// ValidateMountPoints checks that every dir or file declared in mount-points.yaml
// is actually used as a volumeMount.mountPath in at least one pod controller template.
//
// Direction: mount-points.yaml → templates.
//
// Built-in excluded paths: /sys, /dev, /proc — these Linux system paths
// are always available and do not need to be declared in mount-points.yaml.
//
// Module-specific exclusions are configured via dmtlint.yaml:
//
//	templates:
//	  excludeRules:
//	    mount-points:
//	      - /host
//	      - /etc/multipath
func (r *MountPointsRule) ValidateMountPoints(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	dirsByFile := collectMountPointsDirs(m, errorList)
	if len(dirsByFile) == 0 {
		return
	}

	templateMountPaths, hasPodControllers := collectTemplateMountPaths(m, errorList)
	if !hasPodControllers {
		return
	}

	for filePath, dirs := range dirsByFile {
		for _, dir := range dirs {
			normalizedDir := strings.TrimRight(dir, "/")
			if builtinExcludedPaths[normalizedDir] {
				continue
			}

			if !r.Enabled(normalizedDir) {
				continue
			}

			if !templateMountPaths[normalizedDir] {
				errorList.WithFilePath(filePath).
					Warnf("mount-points.yaml references dir %q which is not used as a mountPath in any pod controller", dir)
			}
		}
	}
}

func collectMountPointsDirs(m pkg.Module, errorList *errors.LintRuleErrorsList) map[string][]string {
	dirsByFile := make(map[string][]string)

	modulePath := m.GetPath()
	if modulePath == "" {
		return dirsByFile
	}

	searchDir := filepath.Join(modulePath, "images")
	if _, err := os.Stat(searchDir); os.IsNotExist(err) {
		return dirsByFile
	}

	err := filepath.Walk(modulePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errorList.WithFilePath(path).Errorf("walk error: %s", err)
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Base(path) != "mount-points.yaml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			errorList.Errorf("failed to read %s: %s", path, err)
			return nil
		}

		var mpf mountPointsFile
		if err := yaml.Unmarshal(data, &mpf); err != nil {
			errorList.Errorf("failed to parse %s: %s", path, err)
			return nil
		}

		allEntries := make([]string, 0, len(mpf.Dirs)+len(mpf.Files))
		allEntries = append(allEntries, mpf.Dirs...)
		allEntries = append(allEntries, mpf.Files...)

		if len(allEntries) > 0 {
			dirsByFile[path] = allEntries
		}

		return nil
	})
	if err != nil {
		errorList.Errorf("failed to walk module directory: %s", err)
	}

	return dirsByFile
}

func collectTemplateMountPaths(m pkg.Module, errorList *errors.LintRuleErrorsList) (map[string]bool, bool) {
	mountPaths := make(map[string]bool)
	hasPodControllers := false

	for _, object := range m.GetStorage() {
		if !IsPodController(object.Unstructured.GetKind()) {
			continue
		}

		hasPodControllers = true

		containers, err := object.GetAllContainers()
		if err != nil {
			errorList.WithObjectID(object.Identity()).
				Errorf("failed to get containers for object: %s", err)

			continue
		}

		for _, container := range containers {
			for _, vm := range container.VolumeMounts {
				normalizedPath := strings.TrimRight(vm.MountPath, "/")
				mountPaths[normalizedPath] = true
			}
		}
	}

	return mountPaths, hasPodControllers
}
