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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// objectWithContainerMounts builds a pod controller with one container per entry
// in containers, each carrying the given volumeMount paths. Unlike
// objectWithMounts it lets a test place mounts in distinct containers, which the
// per-container check depends on.
func objectWithContainerMounts(kind, name string, containers map[string][]string) storage.StoreObject {
	specContainers := make([]any, 0, len(containers))

	for containerName, mountPaths := range containers {
		mounts := make([]any, 0, len(mountPaths))
		for _, mp := range mountPaths {
			mounts = append(mounts, map[string]any{
				"name":      "vol",
				"mountPath": mp,
			})
		}

		specContainers = append(specContainers, map[string]any{
			"name":         containerName,
			"image":        "test:latest",
			"volumeMounts": mounts,
		})
	}

	return storage.StoreObject{
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":       kind,
				"apiVersion": "apps/v1",
				"metadata": map[string]any{
					"name":      name,
					"namespace": "default",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": specContainers,
						},
					},
				},
			},
		},
	}
}

func TestSysCgroupMountRule_CheckSysCgroupMount(t *testing.T) {
	tests := []struct {
		name         string
		object       storage.StoreObject
		wantCount    int
		wantContains string
	}{
		{
			name:         "flags a container mounting /sys but not /sys/fs/cgroup",
			object:       objectWithMounts("Deployment", "app", "/sys"),
			wantCount:    1,
			wantContains: `not "/sys/fs/cgroup"`,
		},
		{
			name:      "accepts a container mounting both /sys and /sys/fs/cgroup",
			object:    objectWithMounts("Deployment", "app", "/sys", "/sys/fs/cgroup"),
			wantCount: 0,
		},
		{
			name:      "ignores a container mounting only /sys/fs/cgroup",
			object:    objectWithMounts("Deployment", "app", "/sys/fs/cgroup"),
			wantCount: 0,
		},
		{
			name:      "ignores a container that does not mount /sys",
			object:    objectWithMounts("Deployment", "app", "/etc/app", "/var/data"),
			wantCount: 0,
		},
		{
			name:         "normalizes a trailing slash on /sys",
			object:       objectWithMounts("Deployment", "app", "/sys/"),
			wantCount:    1,
			wantContains: `mounts "/sys"`,
		},
		{
			name:      "fires for a DaemonSet (the classic node-agent case)",
			object:    objectWithMounts("DaemonSet", "node-agent", "/sys"),
			wantCount: 1,
		},
		{
			name:      "skips objects that are not pod controllers",
			object:    objectWithMounts("ConfigMap", "cm", "/sys"),
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containers, err := tt.object.GetAllContainers()
			require.NoError(t, err)

			errorList := errors.NewLintRuleErrorsList()
			NewSysCgroupMountRule(nil, oneObject(tt.object, containers), errorList).Check(t.Context())

			errs := errorList.GetErrors()
			require.Len(t, errs, tt.wantCount)

			if tt.wantContains != "" {
				assert.Truef(t, strings.Contains(errs[0].Text, tt.wantContains),
					"expected finding to contain %q, got %q", tt.wantContains, errs[0].Text)
			}
		})
	}
}

// TestSysCgroupMountRule_PerContainer verifies the check is per container: a
// sibling container mounting /sys/fs/cgroup does not satisfy the requirement for
// the container that actually mounts /sys.
func TestSysCgroupMountRule_PerContainer(t *testing.T) {
	object := objectWithContainerMounts("Deployment", "app", map[string][]string{
		"needs-sys":  {"/sys"},
		"has-cgroup": {"/sys/fs/cgroup"},
	})

	containers, err := object.GetAllContainers()
	require.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	NewSysCgroupMountRule(nil, oneObject(object, containers), errorList).Check(t.Context())

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, "needs-sys")
}

func TestSysCgroupMountRule_Excludes(t *testing.T) {
	object := objectWithMounts("Deployment", "app", "/sys")

	containers, err := object.GetAllContainers()
	require.NoError(t, err)

	t.Run("exclude by kind and name silences the finding", func(t *testing.T) {
		excludes := []pkg.ContainerRuleExclude{{Kind: "Deployment", Name: "app"}}

		errorList := errors.NewLintRuleErrorsList()
		NewSysCgroupMountRule(excludes, oneObject(object, containers), errorList).Check(t.Context())

		assert.Empty(t, errorList.GetErrors())
	})

	t.Run("exclude scoped to the container silences the finding", func(t *testing.T) {
		excludes := []pkg.ContainerRuleExclude{{Kind: "Deployment", Name: "app", Container: "main"}}

		errorList := errors.NewLintRuleErrorsList()
		NewSysCgroupMountRule(excludes, oneObject(object, containers), errorList).Check(t.Context())

		assert.Empty(t, errorList.GetErrors())
	})

	t.Run("exclude for a different object leaves the finding", func(t *testing.T) {
		excludes := []pkg.ContainerRuleExclude{{Kind: "Deployment", Name: "other"}}

		errorList := errors.NewLintRuleErrorsList()
		NewSysCgroupMountRule(excludes, oneObject(object, containers), errorList).Check(t.Context())

		assert.Len(t, errorList.GetErrors(), 1)
	})
}
