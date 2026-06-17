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
	Dirs []string `yaml:"dirs"`
}

func NewMountPointsRule() *MountPointsRule {
	return &MountPointsRule{
		RuleMeta: pkg.RuleMeta{
			Name: MountPointsRuleName,
		},
	}
}

type MountPointsRule struct {
	pkg.RuleMeta
}

func (r *MountPointsRule) ValidateMountPoints(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	dirsByFile := collectMountPointsDirs(m, errorList)
	if len(dirsByFile) == 0 {
		return
	}

	templateMountPaths := collectTemplateMountPaths(m, errorList)
	if len(templateMountPaths) == 0 {
		return
	}

	for filePath, dirs := range dirsByFile {
		for _, dir := range dirs {
			normalizedDir := strings.TrimRight(dir, "/")
			if !templateMountPaths[normalizedDir] {
				errorList.WithFilePath(filePath).
					Errorf("mount-points.yaml references dir %q which is not used as a mountPath in any pod controller", dir)
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
			return err
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

		if len(mpf.Dirs) > 0 {
			dirsByFile[path] = mpf.Dirs
		}

		return nil
	})

	if err != nil {
		errorList.Errorf("failed to walk module directory: %s", err)
	}

	return dirsByFile
}

func collectTemplateMountPaths(m pkg.Module, errorList *errors.LintRuleErrorsList) map[string]bool {
	mountPaths := make(map[string]bool)

	for _, object := range m.GetStorage() {
		if !IsPodController(object.Unstructured.GetKind()) {
			continue
		}

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

	return mountPaths
}
