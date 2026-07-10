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
	"sync"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/storage"
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

func NewMountPointsRule(excludeRules []pkg.StringRuleExclude, modulePath string) *MountPointsRule {
	return &MountPointsRule{
		RuleMeta: pkg.RuleMeta{
			Name: MountPointsRuleName,
		},
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
		mountPointsDirs: collectMountPointsDirs(modulePath),
	}
}

type MountPointsRule struct {
	pkg.RuleMeta
	pkg.StringRule
	mountPointsDirs map[string]bool
}

// CheckMountPaths verifies that every volumeMount.mountPath in pod controllers
// is declared in at least one mount-points.yaml file in the module.
//
// Direction: templates → mount-points.yaml (reverse of the existing templates rule).
func (r *MountPointsRule) CheckMountPaths(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	if len(r.mountPointsDirs) == 0 {
		return
	}

	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet":
	default:
		return
	}

	for _, container := range containers {
		for _, vm := range container.VolumeMounts {
			normalizedPath := strings.TrimRight(vm.MountPath, "/")
			if !r.Enabled(normalizedPath) {
				continue
			}

			if !r.mountPointsDirs[normalizedPath] {
				errorList.WithObjectID(object.Identity()).
					Warnf("Container %q mountPath %q is not declared in any mount-points.yaml", container.Name, vm.MountPath)
			}
		}
	}
}

var mpDirsCache sync.Map // map[string]map[string]bool

// collectMountPointsDirs walks the module directory and collects all dirs
// from mount-points.yaml files into a set keyed by normalized path.
// Results are cached per module path.
func collectMountPointsDirs(modulePath string) map[string]bool {
	if cached, ok := mpDirsCache.Load(modulePath); ok {
		return cached.(map[string]bool)
	}

	dirs := make(map[string]bool)

	searchDir := filepath.Join(modulePath, "images")
	if _, err := os.Stat(searchDir); os.IsNotExist(err) {
		mpDirsCache.Store(modulePath, dirs)
		return dirs
	}

	_ = filepath.Walk(modulePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Base(path) != "mount-points.yaml" {
			return nil
		}

		data, err := fsutils.ReadFile(path)
		if err != nil {
			return nil
		}

		var mpf mountPointsFile
		if err := yaml.Unmarshal(data, &mpf); err != nil {
			return nil
		}

		for _, dir := range mpf.Dirs {
			dirs[strings.TrimRight(dir, "/")] = true
		}

		for _, file := range mpf.Files {
			dirs[strings.TrimRight(file, "/")] = true
		}

		return nil
	})

	mpDirsCache.Store(modulePath, dirs)

	return dirs
}
