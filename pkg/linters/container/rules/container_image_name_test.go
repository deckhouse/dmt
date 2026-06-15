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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "helm-template-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f.Name()
}

func makeStoreObj(t *testing.T, content string) storage.StoreObject {
	t.Helper()

	return storage.StoreObject{
		AbsPath: writeTempFile(t, content),
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "test-obj"},
			},
		},
	}
}

func TestFindObjectRawImages(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedImages []string
	}{
		{
			name:           "no image references",
			fileContent:    "apiVersion: apps/v1\nkind: Deployment\n",
			expectedImages: []string{},
		},
		{
			name:           "single image without underscores",
			fileContent:    `image: {{ include "helm_lib_module_image" . "someImage" }}`,
			expectedImages: []string{"someImage"},
		},
		{
			name: "single image with underscore",
			fileContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-name
  namespace: d8-{{ $.Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" $.Chart.Name)) | nindent 2 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: app-name
  template:
    metadata:
      labels:
        app: app-name
    spec:
      serviceAccountName: {{ $.Chart.Name }}
      automountServiceAccountToken: true
      {{- include "helm_lib_node_selector" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_priority_class" (tuple . "cluster-low") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 6 }}
      containers:
        - name: manager
		  image: {{ include "helm_lib_module_image" . "some_image" }}`,
			expectedImages: []string{"some_image"},
		},
		{
			name: "multiple images mixed",
			fileContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-name
  namespace: d8-{{ $.Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" $.Chart.Name)) | nindent 2 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: app-name
  template:
    metadata:
      labels:
        app: app-name
    spec:
      serviceAccountName: {{ $.Chart.Name }}
      automountServiceAccountToken: true
      {{- include "helm_lib_node_selector" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_priority_class" (tuple . "cluster-low") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 6 }}
      containers:
        - name: manager
		  image: {{ include "helm_lib_module_image" . "firstImage" }}
        - name: manager2
		  image: {{ include "helm_lib_module_image" . "second_image" }}`,
			expectedImages: []string{"firstImage", "second_image"},
		},
		{
			name: "list syntax without underscores",
			fileContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-name
  namespace: d8-{{ $.Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" $.Chart.Name)) | nindent 2 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: app-name
  template:
    metadata:
      labels:
        app: app-name
    spec:
      serviceAccountName: {{ $.Chart.Name }}
      automountServiceAccountToken: true
      {{- include "helm_lib_node_selector" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_priority_class" (tuple . "cluster-low") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 6 }}
      containers:
        - name: manager
		  image: {{ include "helm_lib_module_image" (list . "podReloader") }}`,
			expectedImages: []string{"podReloader"},
		},
		{
			name: "list syntax with underscore",
			fileContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-name
  namespace: d8-{{ $.Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" $.Chart.Name)) | nindent 2 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: app-name
  template:
    metadata:
      labels:
        app: app-name
    spec:
      serviceAccountName: {{ $.Chart.Name }}
      automountServiceAccountToken: true
      {{- include "helm_lib_node_selector" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_priority_class" (tuple . "cluster-low") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 6 }}
      containers:
        - name: manager
		  image: {{ include "helm_lib_module_image" (list . "pod_reloader") }}`,
			expectedImages: []string{"pod_reloader"},
		},
		{
			name: "mixed dot and list syntax",
			fileContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-name
  namespace: d8-{{ $.Chart.Name }}
  {{- include "helm_lib_module_labels" (list . (dict "app" $.Chart.Name)) | nindent 2 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: app-name
  template:
    metadata:
      labels:
        app: app-name
    spec:
      serviceAccountName: {{ $.Chart.Name }}
      automountServiceAccountToken: true
      {{- include "helm_lib_node_selector" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple . "system") | nindent 6 }}
      {{- include "helm_lib_priority_class" (tuple . "cluster-low") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 6 }}
      containers:
        - name: manager
		  image: {{ include "helm_lib_module_image" . "firstImage" }}
		- name: manager2
          image: {{ include "helm_lib_module_image" (list . "second_image") }}`,
			expectedImages: []string{"firstImage", "second_image"},
		},
		{
			name:           "non-matching image line is ignored",
			fileContent:    `image: registry.example.com/myapp:latest`,
			expectedImages: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := makeStoreObj(t, tt.fileContent)
			images, err := FindObjectRawImages(obj.AbsPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedImages, images)
		})
	}
}

func TestFindObjectRawImages_FileNotFound(t *testing.T) {
	obj := storage.StoreObject{
		AbsPath: "/nonexistent/path/file.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "test"},
			},
		},
	}

	images, err := FindObjectRawImages(obj.AbsPath)
	assert.Nil(t, images)
	assert.Error(t, err)
}

func TestImageNoUnderscoresRule_ContainerImageNoUnderscoresCheck(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		containers     []corev1.Container
		expectedErrors []string
	}{
		{
			name:           "no image references in file",
			fileContent:    "apiVersion: apps/v1\nkind: Deployment\n",
			containers:     []corev1.Container{{Name: "test"}},
			expectedErrors: []string{},
		},
		{
			name:           "image without underscores should pass",
			fileContent:    `image: {{ include "helm_lib_module_image" . "validImage" }}`,
			containers:     []corev1.Container{{Name: "test"}},
			expectedErrors: []string{},
		},
		{
			name:           "image with underscore should error",
			fileContent:    `image: {{ include "helm_lib_module_image" . "invalid_image" }}`,
			containers:     []corev1.Container{{Name: "test"}},
			expectedErrors: []string{`Image name "invalid_image" must not contain underscores`},
		},
		{
			name: "multiple images, some with underscores",
			fileContent: `image: {{ include "helm_lib_module_image" . "goodImage" }}
image: {{ include "helm_lib_module_image" . "bad_image" }}
image: {{ include "helm_lib_module_image" . "another_bad_one" }}`,
			containers: []corev1.Container{{Name: "test"}},
			expectedErrors: []string{
				`Image name "bad_image" must not contain underscores`,
				`Image name "another_bad_one" must not contain underscores`,
			},
		},
		{
			name:           "camelCase image name should pass",
			fileContent:    `image: {{ include "helm_lib_module_image" . "myAppContainer" }}`,
			containers:     []corev1.Container{{Name: "test"}},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewContainerImageNameRule([]pkg.ContainerRuleExclude{})
			errorList := errors.NewLintRuleErrorsList()

			obj := makeStoreObj(t, tt.fileContent)

			rule.ContainerImageNameCheck(obj, tt.containers, errorList)
			errs := errorList.GetErrors()

			if len(tt.expectedErrors) == 0 {
				assert.Empty(t, errs, "Expected no errors")
			} else {
				assert.Len(t, errs, len(tt.expectedErrors), "Expected %d errors", len(tt.expectedErrors))

				for i, expectedError := range tt.expectedErrors {
					assert.Contains(t, errs[i].Text, expectedError, "Error %d should contain expected text", i)
				}
			}
		})
	}
}

func TestImageNoUnderscoresRule_FileReadError(t *testing.T) {
	rule := NewContainerImageNameRule([]pkg.ContainerRuleExclude{})
	errorList := errors.NewLintRuleErrorsList()

	obj := storage.StoreObject{
		AbsPath: "/nonexistent/path/file.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "test"},
			},
		},
	}

	rule.ContainerImageNameCheck(obj, []corev1.Container{{Name: "test"}}, errorList)
	errs := errorList.GetErrors()

	assert.Len(t, errs, 1, "Should have one error for file read failure")
	assert.Contains(t, errs[0].Text, "Failed to read images from template file")
}

func TestImageNoUnderscoresRule_Enabled(t *testing.T) {
	excludeRules := []pkg.ContainerRuleExclude{
		{
			Kind:      "Deployment",
			Name:      "excluded-deployment",
			Container: "excluded-container",
		},
	}

	rule := NewContainerImageNameRule(excludeRules)

	excludedObj := storage.StoreObject{
		AbsPath: "test.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "excluded-deployment"},
			},
		},
	}

	otherObj := storage.StoreObject{
		AbsPath: "test.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "other-deployment"},
			},
		},
	}

	excludedContainer := &corev1.Container{Name: "excluded-container"}
	otherContainer := &corev1.Container{Name: "other-container"}

	assert.False(t, rule.Enabled(excludedObj, excludedContainer), "Excluded object+container should not be enabled")
	assert.True(t, rule.Enabled(excludedObj, otherContainer), "Excluded object with different container should be enabled")
	assert.True(t, rule.Enabled(otherObj, excludedContainer), "Other object with excluded container name should be enabled")
	assert.True(t, rule.Enabled(otherObj, otherContainer), "Non-excluded object+container should be enabled")
}
