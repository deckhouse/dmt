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
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func deploymentWithMounts(name string, mountPaths ...string) storage.StoreObject {
	volumeMounts := make([]any, 0, len(mountPaths))
	for _, mp := range mountPaths {
		volumeMounts = append(volumeMounts, map[string]any{
			"name":      "vol-" + name,
			"mountPath": mp,
		})
	}

	return storage.StoreObject{
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":       "Deployment",
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
									"name":         "main",
									"image":        "test:latest",
									"volumeMounts": volumeMounts,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestNewMountPointsRule(t *testing.T) {
	rule := NewMountPointsRule(nil)
	assert.Equal(t, MountPointsRuleName, rule.Name)
}

func TestMountPointsRule_AllDirsMatched(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/app
  - /etc/app/certs
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithMounts("app", "/etc/app", "/etc/app/certs"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_MissingDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/app
  - /etc/not-mounted
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithMounts("app", "/etc/app"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 1)
	assert.Equal(t, pkg.Warn, errs[0].Level)
	assert.Contains(t, errs[0].Text, `"/etc/not-mounted"`)
}

func TestMountPointsRule_MultipleFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	for _, img := range []string{"app1", "app2"} {
		imgDir := filepath.Join(tmpDir, "images", img)
		if err := os.MkdirAll(imgDir, 0755); err != nil {
			t.Fatal(err)
		}

		mountPointsYAML := `dirs:
  - /etc/` + img + `
`
		if err := os.WriteFile(filepath.Join(imgDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
			t.Fatal(err)
		}
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app1", Namespace: "default"}: deploymentWithMounts("app1", "/etc/app1"),
		{Kind: "Deployment", Name: "app2", Namespace: "default"}: deploymentWithMounts("app2", "/etc/app2"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_NoMountPointsFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithMounts("app", "/etc/app"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_EmptyMountPointsFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs: []
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithMounts("app", "/etc/app"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_NoPodControllers_ReportsUnusedDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/app
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: nil}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 1)
	assert.Equal(t, pkg.Warn, errs[0].Level)
	assert.Contains(t, errs[0].Text, `"/etc/app"`)
}

func TestMountPointsRule_DaemonSetAndStatefulSet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/daemon
  - /etc/sts
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	dsMounts := []any{
		map[string]any{"name": "vol-ds", "mountPath": "/etc/daemon"},
	}
	dsObj := storage.StoreObject{
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":       "DaemonSet",
				"apiVersion": "apps/v1",
				"metadata":   map[string]any{"name": "ds", "namespace": "default"},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{"name": "main", "image": "test:latest", "volumeMounts": dsMounts},
							},
						},
					},
				},
			},
		},
	}

	stsMounts := []any{
		map[string]any{"name": "vol-sts", "mountPath": "/etc/sts"},
	}
	stsObj := storage.StoreObject{
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":       "StatefulSet",
				"apiVersion": "apps/v1",
				"metadata":   map[string]any{"name": "sts", "namespace": "default"},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{"name": "main", "image": "test:latest", "volumeMounts": stsMounts},
							},
						},
					},
				},
			},
		},
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "DaemonSet", Name: "ds", Namespace: "default"}:    dsObj,
		{Kind: "StatefulSet", Name: "sts", Namespace: "default"}: stsObj,
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_TrailingSlashMatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/app/
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithMounts("app", "/etc/app"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_InitContainers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/init
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: {
			Unstructured: unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Deployment",
					"apiVersion": "apps/v1",
					"metadata":   map[string]any{"name": "app", "namespace": "default"},
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []map[string]any{
									{"name": "main", "image": "test:latest"},
								},
								"initContainers": []map[string]any{
									{
										"name":  "init",
										"image": "init:latest",
										"volumeMounts": []map[string]any{
											{"name": "vol-init", "mountPath": "/etc/init"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_ExcludeDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/app
  - /etc/not-mounted
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithMounts("app", "/etc/app"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule([]pkg.StringRuleExclude{"/etc/not-mounted"})
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_ExcludeDirWithTrailingSlash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/app
  - /etc/not-mounted/
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithMounts("app", "/etc/app"),
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule([]pkg.StringRuleExclude{"/etc/not-mounted"})
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 0)
}

func TestMountPointsRule_ControllerWithoutVolumeMounts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount-points-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	imagesDir := filepath.Join(tmpDir, "images", "app")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	mountPointsYAML := `dirs:
  - /etc/app
`
	if err := os.WriteFile(filepath.Join(imagesDir, "mount-points.yaml"), []byte(mountPointsYAML), 0600); err != nil {
		t.Fatal(err)
	}

	deploymentWithNoMounts := storage.StoreObject{
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":       "Deployment",
				"apiVersion": "apps/v1",
				"metadata": map[string]any{
					"name":      "app",
					"namespace": "default",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name":  "main",
									"image": "test:latest",
								},
							},
						},
					},
				},
			},
		},
	}

	storageMap := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "app", Namespace: "default"}: deploymentWithNoMounts,
	}

	errorList := errors.NewLintRuleErrorsList()
	rule := NewMountPointsRule(nil)
	rule.ValidateMountPoints(&mockMountPointsModule{path: tmpDir, storage: storageMap}, errorList)

	errs := errorList.GetErrors()
	assert.Len(t, errs, 1)
	assert.Equal(t, pkg.Warn, errs[0].Level)
	assert.Contains(t, errs[0].Text, `"/etc/app"`)
}

type mockMountPointsModule struct {
	path    string
	storage map[storage.ResourceIndex]storage.StoreObject
}

func (m *mockMountPointsModule) GetName() string                                  { return "test-module" }
func (m *mockMountPointsModule) GetNamespace() string                             { return "default" }
func (m *mockMountPointsModule) GetPath() string                                  { return m.path }
func (m *mockMountPointsModule) GetWerfFile() string                              { return "" }
func (m *mockMountPointsModule) GetChart() *chart.Chart                           { return nil }
func (m *mockMountPointsModule) GetObjectStore() *storage.UnstructuredObjectStore { return nil }
func (m *mockMountPointsModule) GetStorage() map[storage.ResourceIndex]storage.StoreObject {
	return m.storage
}
