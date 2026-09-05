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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// readOnlyObject builds the rendered object the rule inspects. Only kind and
// name matter to it: the containers arrive separately, already extracted.
func readOnlyObject(kind, name string) storage.StoreObject {
	return storage.StoreObject{
		AbsPath: "test.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     kind,
				"metadata": map[string]any{"name": name},
			},
		},
	}
}

func TestCheckReadOnlyRootFilesystemRule_ContainerReadOnlyRootFilesystem(t *testing.T) {
	tests := []struct {
		name           string
		kind           string
		containers     []corev1.Container
		expectedErrors []string
	}{
		{
			name: "unsupported kind should be ignored",
			kind: "Service",
			containers: []corev1.Container{{
				Name: "test",
			}},
			expectedErrors: []string{},
		},
		{
			name: "missing security context should error",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
			}},
			expectedErrors: []string{
				"Container's SecurityContext is missing",
			},
		},
		{
			name: "missing readOnlyRootFilesystem should error",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot: boolPtr(true),
				},
			}},
			expectedErrors: []string{
				"Container's SecurityContext missing parameter ReadOnlyRootFilesystem",
			},
		},
		{
			name: "readOnlyRootFilesystem false should error",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					ReadOnlyRootFilesystem: boolPtr(false),
				},
			}},
			expectedErrors: []string{
				"Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`",
			},
		},
		{
			name: "readOnlyRootFilesystem true should pass",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					ReadOnlyRootFilesystem: boolPtr(true),
				},
			}},
			expectedErrors: []string{},
		},
		{
			name: "multiple containers with mixed settings",
			kind: "Pod",
			containers: []corev1.Container{
				{
					Name: "good-container",
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: boolPtr(true),
					},
				},
				{
					Name: "bad-container",
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: boolPtr(false),
					},
				},
				{
					Name: "missing-parameter",
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: nil,
					},
				},
				{
					Name: "missing-context",
				},
			},
			expectedErrors: []string{
				"Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`",
				"Container's SecurityContext missing parameter ReadOnlyRootFilesystem",
				"Container's SecurityContext is missing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()

			obj := readOnlyObject(tt.kind, "test-obj")

			NewCheckReadOnlyRootFilesystemRule([]pkg.ContainerRuleExclude{}, oneObject(obj, tt.containers), errorList).Check(t.Context())
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

// TestCheckReadOnlyRootFilesystemRule_Kinds pins the kind gate: the six workload
// kinds the rule claims are checked, and anything else — ReplicaSet included,
// even though container extraction supports it — is passed over.
func TestCheckReadOnlyRootFilesystemRule_Kinds(t *testing.T) {
	checked := []string{"Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob"}
	skipped := []string{"ReplicaSet", "Service", "ConfigMap", "CustomResourceDefinition", ""}

	// A container that fails every stage of the check, so the only reason for
	// silence can be the kind gate.
	failing := []corev1.Container{{
		Name: "test-container",
		SecurityContext: &corev1.SecurityContext{
			ReadOnlyRootFilesystem: boolPtr(false),
		},
	}}

	for _, kind := range checked {
		t.Run("checked/"+kind, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()

			NewCheckReadOnlyRootFilesystemRule(
				[]pkg.ContainerRuleExclude{},
				oneObject(readOnlyObject(kind, "test-obj"), failing),
				errorList,
			).Check(t.Context())

			assert.Len(t, errorList.GetErrors(), 1, "%s must be checked", kind)
		})
	}

	for _, kind := range skipped {
		t.Run("skipped/"+kind, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()

			NewCheckReadOnlyRootFilesystemRule(
				[]pkg.ContainerRuleExclude{},
				oneObject(readOnlyObject(kind, "test-obj"), failing),
				errorList,
			).Check(t.Context())

			assert.Empty(t, errorList.GetErrors(), "%q must not be checked", kind)
		})
	}
}

// TestCheckReadOnlyRootFilesystemRule_InitContainers guards that init containers
// are held to the same requirement: the rule reads the All slice, which carries
// regular and init containers together.
func TestCheckReadOnlyRootFilesystemRule_InitContainers(t *testing.T) {
	regular := corev1.Container{
		Name: "app",
		SecurityContext: &corev1.SecurityContext{
			ReadOnlyRootFilesystem: boolPtr(true),
		},
	}
	init := corev1.Container{
		Name: "init-app",
		SecurityContext: &corev1.SecurityContext{
			ReadOnlyRootFilesystem: boolPtr(false),
		},
	}

	errorList := errors.NewLintRuleErrorsList()

	objects := []ObjectContainers{{
		Object:  readOnlyObject("Deployment", "test-obj"),
		All:     []corev1.Container{regular, init},
		NotInit: []corev1.Container{regular},
	}}

	NewCheckReadOnlyRootFilesystemRule([]pkg.ContainerRuleExclude{}, objects, errorList).Check(t.Context())
	errs := errorList.GetErrors()

	assert.Len(t, errs, 1, "The failing init container must be reported")
	assert.Contains(t, errs[0].Text, "Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`")
	assert.Contains(t, errs[0].ObjectID, "container = init-app")
}

// TestCheckReadOnlyRootFilesystemRule_SkippedObjectDoesNotStopRule is the
// regression guard for the early returns in checkObject: they must end the
// check for one object, not for the whole rule. An unsupported kind and an
// object with no containers both come first, so an inlined early return would
// hide the failing Deployment behind them.
func TestCheckReadOnlyRootFilesystemRule_SkippedObjectDoesNotStopRule(t *testing.T) {
	failing := []corev1.Container{{
		Name: "test-container",
		SecurityContext: &corev1.SecurityContext{
			ReadOnlyRootFilesystem: boolPtr(false),
		},
	}}

	errorList := errors.NewLintRuleErrorsList()

	objects := []ObjectContainers{
		{Object: readOnlyObject("Service", "some-service"), All: failing, NotInit: failing},
		{Object: readOnlyObject("Deployment", "empty-deployment")},
		{Object: readOnlyObject("Deployment", "failing-deployment"), All: failing, NotInit: failing},
	}

	NewCheckReadOnlyRootFilesystemRule([]pkg.ContainerRuleExclude{}, objects, errorList).Check(t.Context())
	errs := errorList.GetErrors()

	assert.Len(t, errs, 1, "The failing Deployment must still be reported")
	assert.Contains(t, errs[0].ObjectID, "failing-deployment")
}

func TestCheckReadOnlyRootFilesystemRule_WithExclusions(t *testing.T) {
	failing := corev1.Container{
		Name: "excluded-container",
		SecurityContext: &corev1.SecurityContext{
			ReadOnlyRootFilesystem: boolPtr(false), // This would normally fail
		},
	}

	tests := []struct {
		name         string
		excludeRules []pkg.ContainerRuleExclude
		kind         string
		objectName   string
		containers   []corev1.Container
		expectErrors int
	}{
		{
			name: "matching kind, name and container is excluded",
			excludeRules: []pkg.ContainerRuleExclude{{
				Kind:      "Deployment",
				Name:      "excluded-deployment",
				Container: "excluded-container",
			}},
			kind:         "Deployment",
			objectName:   "excluded-deployment",
			containers:   []corev1.Container{failing},
			expectErrors: 0,
		},
		{
			name: "empty container field excludes every container of the object",
			excludeRules: []pkg.ContainerRuleExclude{{
				Kind: "Deployment",
				Name: "excluded-deployment",
			}},
			kind:       "Deployment",
			objectName: "excluded-deployment",
			containers: []corev1.Container{
				failing,
				{
					Name: "another-container",
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: boolPtr(false),
					},
				},
			},
			expectErrors: 0,
		},
		{
			name: "exclusion for another container still reports this one",
			excludeRules: []pkg.ContainerRuleExclude{{
				Kind:      "Deployment",
				Name:      "excluded-deployment",
				Container: "some-other-container",
			}},
			kind:         "Deployment",
			objectName:   "excluded-deployment",
			containers:   []corev1.Container{failing},
			expectErrors: 1,
		},
		{
			name: "exclusion for another object name still reports",
			excludeRules: []pkg.ContainerRuleExclude{{
				Kind:      "Deployment",
				Name:      "some-other-deployment",
				Container: "excluded-container",
			}},
			kind:         "Deployment",
			objectName:   "excluded-deployment",
			containers:   []corev1.Container{failing},
			expectErrors: 1,
		},
		{
			name: "exclusion for another kind still reports",
			excludeRules: []pkg.ContainerRuleExclude{{
				Kind:      "DaemonSet",
				Name:      "excluded-deployment",
				Container: "excluded-container",
			}},
			kind:         "Deployment",
			objectName:   "excluded-deployment",
			containers:   []corev1.Container{failing},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()

			obj := readOnlyObject(tt.kind, tt.objectName)

			NewCheckReadOnlyRootFilesystemRule(tt.excludeRules, oneObject(obj, tt.containers), errorList).Check(t.Context())

			assert.Len(t, errorList.GetErrors(), tt.expectErrors)
		})
	}
}
