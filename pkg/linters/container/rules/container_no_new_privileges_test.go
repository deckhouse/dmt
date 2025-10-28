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

func TestNoNewPrivilegesRule_ContainerNoNewPrivileges(t *testing.T) {
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
				"Container's SecurityContext is missing - cannot verify allowPrivilegeEscalation setting",
			},
		},
		{
			name: "missing allowPrivilegeEscalation should error",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot: boolPtr(true),
				},
			}},
			expectedErrors: []string{
				"Container's SecurityContext missing parameter AllowPrivilegeEscalation - should be set to false to prevent privilege escalation",
			},
		},
		{
			name: "allowPrivilegeEscalation true should error",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(true),
				},
			}},
			expectedErrors: []string{
				"Container's SecurityContext has `AllowPrivilegeEscalation: true`, but it must be `false` to prevent privilege escalation attacks",
			},
		},
		{
			name: "allowPrivilegeEscalation false should pass",
			kind: "Deployment",
			containers: []corev1.Container{{
				Name: "test-container",
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(false),
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
						AllowPrivilegeEscalation: boolPtr(false),
					},
				},
				{
					Name: "bad-container",
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(true),
					},
				},
				{
					Name: "missing-context",
				},
			},
			expectedErrors: []string{
				"Container's SecurityContext has `AllowPrivilegeEscalation: true`, but it must be `false` to prevent privilege escalation attacks",
				"Container's SecurityContext is missing - cannot verify allowPrivilegeEscalation setting",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewNoNewPrivilegesRule([]pkg.ContainerRuleExclude{})
			errorList := errors.NewLintRuleErrorsList()

			obj := storage.StoreObject{
				AbsPath: "test.yaml",
				Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"kind":     tt.kind,
						"metadata": map[string]any{"name": "test-obj"},
					},
				},
			}

			rule.ContainerNoNewPrivileges(obj, tt.containers, errorList)
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

func TestNoNewPrivilegesRule_WithExclusions(t *testing.T) {
	excludeRules := []pkg.ContainerRuleExclude{
		{
			Kind:      "Deployment",
			Name:      "excluded-deployment",
			Container: "excluded-container",
		},
	}

	rule := NewNoNewPrivilegesRule(excludeRules)
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
			AllowPrivilegeEscalation: boolPtr(true), // This would normally fail
		},
	}}

	rule.ContainerNoNewPrivileges(obj, containers, errorList)
	errs := errorList.GetErrors()

	assert.Empty(t, errs, "Excluded container should not generate errors")
}

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}
