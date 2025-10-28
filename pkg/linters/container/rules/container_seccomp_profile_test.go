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

func TestSeccompProfileRule_ContainerSeccompProfile(t *testing.T) {
	tests := []struct {
		name             string
		kind             string
		podSpec          map[string]any
		containers       []corev1.Container
		expectedMessages []string
	}{
		{
			name:             "unsupported kind should be ignored",
			kind:             "Service",
			containers:       []corev1.Container{{Name: "test"}},
			expectedMessages: []string{},
		},
		{
			name: "no seccomp profile should warn",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
			}},
			expectedMessages: []string{
				"No seccomp profile specified - consider explicitly setting seccompProfile.type to 'RuntimeDefault' for better security posture",
			},
		},
		{
			name: "RuntimeDefault profile should pass",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			}},
			expectedMessages: []string{},
		},
		{
			name: "Unconfined profile should error",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeUnconfined,
					},
				},
			}},
			expectedMessages: []string{
				"Container has seccompProfile.type set to 'Unconfined' which disables seccomp filtering and poses security risks - use 'RuntimeDefault' instead",
			},
		},
		{
			name: "Localhost profile with valid path should warn",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type:             corev1.SeccompProfileTypeLocalhost,
						LocalhostProfile: stringPtr("/path/to/profile.json"),
					},
				},
			}},
			expectedMessages: []string{
				"Container uses custom seccomp profile - ensure it's properly configured and maintained",
			},
		},
		{
			name: "Localhost profile without path should error",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeLocalhost,
					},
				},
			}},
			expectedMessages: []string{
				"Container has seccompProfile.type set to 'Localhost' but localhostProfile is empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewSeccompProfileRule([]pkg.ContainerRuleExclude{})
			errorList := errors.NewLintRuleErrorsList()

			objData := map[string]any{
				"kind":     tt.kind,
				"metadata": map[string]any{"name": "test-obj"},
			}

			if tt.podSpec != nil {
				objData["spec"] = map[string]any{
					"template": map[string]any{
						"spec": tt.podSpec,
					},
				}
			}

			obj := storage.StoreObject{
				AbsPath:      "test.yaml",
				Unstructured: unstructured.Unstructured{Object: objData},
			}

			rule.ContainerSeccompProfile(obj, tt.containers, errorList)
			errs := errorList.GetErrors()

			if len(tt.expectedMessages) == 0 {
				assert.Empty(t, errs, "Expected no messages")
			} else {
				assert.Len(t, errs, len(tt.expectedMessages), "Expected %d messages", len(tt.expectedMessages))
				for i, expectedMessage := range tt.expectedMessages {
					assert.Contains(t, errs[i].Text, expectedMessage, "Message %d should contain expected text", i)
				}
			}
		})
	}
}

func TestSeccompProfileRule_PodLevelProfile(t *testing.T) {
	rule := NewSeccompProfileRule([]pkg.ContainerRuleExclude{})
	errorList := errors.NewLintRuleErrorsList()

	// Pod with RuntimeDefault at pod level
	objData := map[string]any{
		"kind":     "Pod",
		"metadata": map[string]any{"name": "test-pod"},
		"spec": map[string]any{
			"securityContext": map[string]any{
				"seccompProfile": map[string]any{
					"type": "RuntimeDefault",
				},
			},
		},
	}

	obj := storage.StoreObject{
		AbsPath:      "test.yaml",
		Unstructured: unstructured.Unstructured{Object: objData},
	}

	containers := []corev1.Container{{
		Name: "test-container",
		// No container-level seccomp profile, should inherit from pod
	}}

	rule.ContainerSeccompProfile(obj, containers, errorList)
	errs := errorList.GetErrors()

	// Should pass without errors since pod-level RuntimeDefault is inherited
	assert.Empty(t, errs, "Pod-level RuntimeDefault should be inherited and pass validation")
}

func TestSeccompProfileRule_ContainerOverridesPod(t *testing.T) {
	rule := NewSeccompProfileRule([]pkg.ContainerRuleExclude{})
	errorList := errors.NewLintRuleErrorsList()

	// Pod with RuntimeDefault, but container overrides with Unconfined
	objData := map[string]any{
		"kind":     "Pod",
		"metadata": map[string]any{"name": "test-pod"},
		"spec": map[string]any{
			"securityContext": map[string]any{
				"seccompProfile": map[string]any{
					"type": "RuntimeDefault",
				},
			},
		},
	}

	obj := storage.StoreObject{
		AbsPath:      "test.yaml",
		Unstructured: unstructured.Unstructured{Object: objData},
	}

	containers := []corev1.Container{{
		Name: "test-container",
		SecurityContext: &corev1.SecurityContext{
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeUnconfined,
			},
		},
	}}

	rule.ContainerSeccompProfile(obj, containers, errorList)
	errs := errorList.GetErrors()

	assert.Len(t, errs, 1, "Should have one error for Unconfined override")
	assert.Contains(t, errs[0].Text, "Unconfined", "Should error about Unconfined profile")
}

func TestSeccompProfileRule_WithExclusions(t *testing.T) {
	excludeRules := []pkg.ContainerRuleExclude{
		{
			Kind:      "Deployment",
			Name:      "excluded-deployment",
			Container: "excluded-container",
		},
	}

	rule := NewSeccompProfileRule(excludeRules)
	errorList := errors.NewLintRuleErrorsList()

	obj := storage.StoreObject{
		AbsPath: "test.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "excluded-deployment"},
			},
		},
	}

	containers := []corev1.Container{{
		Name: "excluded-container",
		SecurityContext: &corev1.SecurityContext{
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeUnconfined, // Would normally fail
			},
		},
	}}

	rule.ContainerSeccompProfile(obj, containers, errorList)
	errs := errorList.GetErrors()

	assert.Empty(t, errs, "Excluded container should not generate errors")
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
