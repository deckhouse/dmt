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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func objectWithMounts(kind, name string, mountPaths ...string) storage.StoreObject {
	volumeMounts := make([]corev1.VolumeMount, 0, len(mountPaths))
	for _, mp := range mountPaths {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "vol",
			MountPath: mp,
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
							"containers": []map[string]any{
								{
									"name":  "main",
									"image": "test:latest",
									"volumeMounts": func() []any {
										mounts := make([]any, 0, len(volumeMounts))
										for _, vm := range volumeMounts {
											mounts = append(mounts, map[string]any{
												"name":      vm.Name,
												"mountPath": vm.MountPath,
											})
										}

										return mounts
									}(),
								},
							},
						},
					},
				},
			},
		},
	}
}

func writeMountPointsFile(t *testing.T, dir, content string) {
	t.Helper()

	imagesDir := filepath.Join(dir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMountPointsContainerRule_AllDeclared(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpcr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	writeMountPointsFile(t, tmpDir, `dirs:
  - /etc/app
  - /etc/app/certs
`)

	obj := objectWithMounts("Deployment", "app", "/etc/app", "/etc/app/certs")

	containers, err := obj.GetAllContainers()
	assert.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil, tmpDir)
	rule.CheckMountPaths(obj, containers, errorList)

	assert.Len(t, errorList.GetErrors(), 0)
}

func TestMountPointsContainerRule_MissingDeclared(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpcr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	writeMountPointsFile(t, tmpDir, `dirs:
  - /etc/app
`)

	obj := objectWithMounts("Deployment", "app", "/etc/app", "/etc/missing")

	containers, err := obj.GetAllContainers()
	assert.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil, tmpDir)
	rule.CheckMountPaths(obj, containers, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 1)
	assert.Equal(t, pkg.Warn, errs[0].Level)
	assert.Contains(t, errs[0].Text, "not declared in any mount-points.yaml")
}

func TestMountPointsContainerRule_NoMountPointsFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpcr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	obj := objectWithMounts("Deployment", "app", "/etc/app", "/etc/missing")

	containers, err := obj.GetAllContainers()
	assert.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil, tmpDir)
	rule.CheckMountPaths(obj, containers, errorList)

	assert.Len(t, errorList.GetErrors(), 0)
}

func TestMountPointsContainerRule_ExcludedPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpcr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	writeMountPointsFile(t, tmpDir, `dirs:
  - /etc/app
`)

	obj := objectWithMounts("Deployment", "app", "/etc/app", "/etc/excluded")

	containers, err := obj.GetAllContainers()
	assert.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule([]pkg.StringRuleExclude{"/etc/excluded"}, tmpDir)
	rule.CheckMountPaths(obj, containers, errorList)

	assert.Len(t, errorList.GetErrors(), 0)
}

func TestMountPointsContainerRule_NonPodController(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpcr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	writeMountPointsFile(t, tmpDir, `dirs:
  - /etc/app
`)

	obj := objectWithMounts("ConfigMap", "app", "/etc/not-checked")

	containers, err := obj.GetAllContainers()
	assert.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil, tmpDir)
	rule.CheckMountPaths(obj, containers, errorList)

	assert.Len(t, errorList.GetErrors(), 0)
}

func TestMountPointsContainerRule_TrailingSlash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpcr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	writeMountPointsFile(t, tmpDir, `dirs:
  - /etc/app/
`)

	obj := objectWithMounts("Deployment", "app", "/etc/app")

	containers, err := obj.GetAllContainers()
	assert.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil, tmpDir)
	rule.CheckMountPaths(obj, containers, errorList)

	assert.Len(t, errorList.GetErrors(), 0)
}

func TestMountPointsContainerRule_DaemonSetAndStatefulSet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mpcr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	writeMountPointsFile(t, tmpDir, `dirs:
  - /etc/daemon
  - /etc/sts
`)

	dsObj := objectWithMounts("DaemonSet", "ds", "/etc/daemon")
	dsContainers, err := dsObj.GetAllContainers()
	assert.NoError(t, err)

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil, tmpDir)
	rule.CheckMountPaths(dsObj, dsContainers, errorList)
	assert.Len(t, errorList.GetErrors(), 0)

	stsObj := objectWithMounts("StatefulSet", "sts", "/etc/sts")
	stsContainers, err := stsObj.GetAllContainers()
	assert.NoError(t, err)

	errorList = errors.NewLintRuleErrorsList()
	rule = NewMountPointsRule(nil, tmpDir)
	rule.CheckMountPaths(stsObj, stsContainers, errorList)
	assert.Len(t, errorList.GetErrors(), 0)
}
