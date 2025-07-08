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

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetResourceIndex(t *testing.T) {
	obj := StoreObject{
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "test-deployment", "namespace": "test-namespace"},
			},
		},
	}
	index := GetResourceIndex(obj)
	assert.Equal(t, "Deployment", index.Kind)
	assert.Equal(t, "test-deployment", index.Name)
	assert.Equal(t, "test-namespace", index.Namespace)
}

func TestResourceIndex_AsString(t *testing.T) {
	tests := []struct {
		name     string
		index    ResourceIndex
		expected string
	}{
		{
			name:     "With namespace",
			index:    ResourceIndex{Kind: "Deployment", Name: "test-deployment", Namespace: "test-namespace"},
			expected: "test-namespace/Deployment/test-deployment",
		},
		{
			name:     "Without namespace",
			index:    ResourceIndex{Kind: "Deployment", Name: "test-deployment"},
			expected: "Deployment/test-deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.index.AsString())
		})
	}
}

func TestStoreObject_ShortPath(t *testing.T) {
	obj := StoreObject{Path: "/module/templates/test.yaml"}
	assert.Equal(t, "module/templates/test.yaml", obj.ShortPath())
}

func TestStoreObject_Identity(t *testing.T) {
	obj := StoreObject{
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Deployment",
				"metadata": map[string]any{"name": "test-deployment", "namespace": "test-namespace"},
			},
		},
	}
	assert.Equal(t, "kind = Deployment ; name = test-deployment ; namespace = test-namespace", obj.Identity())
}

func TestUnstructuredObjectStore_PutAndGet(t *testing.T) {
	store := NewUnstructuredObjectStore()
	obj := map[string]any{
		"kind":     "Deployment",
		"metadata": map[string]any{"name": "test-deployment", "namespace": "test-namespace"},
	}
	raw := []byte(`kind: Deployment
metadata:
  name: test-deployment
  namespace: test-namespace`)

	err := store.Put("/path/to/object.yaml", obj, raw)
	require.NoError(t, err)

	index := ResourceIndex{Kind: "Deployment", Name: "test-deployment", Namespace: "test-namespace"}
	storedObj := store.Get(index)
	assert.Equal(t, "test-deployment", storedObj.Unstructured.GetName())
	assert.Equal(t, "test-namespace", storedObj.Unstructured.GetNamespace())
	assert.Equal(t, NewSHA256(raw), storedObj.Hash)
}

func TestUnstructuredObjectStore_Exists(t *testing.T) {
	store := NewUnstructuredObjectStore()
	index := ResourceIndex{Kind: "Deployment", Name: "test-deployment", Namespace: "test-namespace"}
	assert.False(t, store.Exists(index))

	obj := map[string]any{
		"kind":     "Deployment",
		"metadata": map[string]any{"name": "test-deployment", "namespace": "test-namespace"},
	}
	raw := []byte(`kind: Deployment
metadata:
  name: test-deployment
  namespace: test-namespace`)

	err := store.Put("/path/to/object.yaml", obj, raw)
	require.NoError(t, err)
	assert.True(t, store.Exists(index))
}

func TestUnstructuredObjectStore_Close(t *testing.T) {
	store := NewUnstructuredObjectStore()
	obj := map[string]any{
		"kind":     "Deployment",
		"metadata": map[string]any{"name": "test-deployment", "namespace": "test-namespace"},
	}
	raw := []byte(`kind: Deployment
metadata:
  name: test-deployment
  namespace: test-namespace`)

	err := store.Put("/path/to/object.yaml", obj, raw)
	require.NoError(t, err)

	store.Close()
	assert.Empty(t, store.Storage)
}

func TestNewSHA256(t *testing.T) {
	data := []byte("test data")
	expectedHash := "916f0027a575074ce72a331777c3478d6513f786a591bd892da1a577bf2335f9"
	assert.Equal(t, expectedHash, NewSHA256(data))
}

func TestStoreObject_GetInitContainers(t *testing.T) {
	tests := []struct {
		name        string
		object      map[string]any
		expected    []string
		expectError bool
	}{
		{
			name: "Deployment with init containers",
			object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"initContainers": []any{
								map[string]any{"name": "init-container-1"},
								map[string]any{"name": "init-container-2"},
							},
						},
					},
				},
			},
			expected:    []string{"init-container-1", "init-container-2"},
			expectError: false,
		},
		{
			name: "Pod with init containers",
			object: map[string]any{
				"kind": "Pod",
				"spec": map[string]any{
					"initContainers": []any{
						map[string]any{"name": "init-container-1"},
					},
				},
			},
			expected:    []string{"init-container-1"},
			expectError: false,
		},
		{
			name: "StatefulSet without init containers",
			object: map[string]any{
				"kind": "StatefulSet",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{},
					},
				},
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "Invalid kind",
			object: map[string]any{
				"kind": "UnknownKind",
			},
			expected:    nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: tt.object,
				},
			}

			initContainers, err := obj.GetInitContainers()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				var containerNames []string
				for _, container := range initContainers {
					containerNames = append(containerNames, container.Name)
				}
				assert.Equal(t, tt.expected, containerNames)
			}
		})
	}
}

func TestStoreObject_GetContainers(t *testing.T) {
	tests := []struct {
		name        string
		object      map[string]any
		expected    []string
		expectError bool
	}{
		{
			name: "Deployment with containers",
			object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{"name": "container-1"},
								map[string]any{"name": "container-2"},
							},
						},
					},
				},
			},
			expected:    []string{"container-1", "container-2"},
			expectError: false,
		},
		{
			name: "Pod with containers",
			object: map[string]any{
				"kind": "Pod",
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "container-1"},
					},
				},
			},
			expected:    []string{"container-1"},
			expectError: false,
		},
		{
			name: "StatefulSet without containers",
			object: map[string]any{
				"kind": "StatefulSet",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{},
					},
				},
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "Invalid kind",
			object: map[string]any{
				"kind": "UnknownKind",
			},
			expected:    nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: tt.object,
				},
			}

			containers, err := obj.GetContainers()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				var containerNames []string
				for _, container := range containers {
					containerNames = append(containerNames, container.Name)
				}
				assert.Equal(t, tt.expected, containerNames)
			}
		})
	}
}

func TestStoreObject_GetAllContainers(t *testing.T) {
	tests := []struct {
		name        string
		object      map[string]any
		expected    []string
		expectError bool
	}{
		{
			name: "Deployment with containers and init containers",
			object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{"name": "container-1"},
								map[string]any{"name": "container-2"},
							},
							"initContainers": []any{
								map[string]any{"name": "init-container-1"},
							},
						},
					},
				},
			},
			expected:    []string{"container-1", "container-2", "init-container-1"},
			expectError: false,
		},
		{
			name: "Pod with only containers",
			object: map[string]any{
				"kind": "Pod",
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "container-1"},
					},
				},
			},
			expected:    []string{"container-1"},
			expectError: false,
		},
		{
			name: "StatefulSet with only init containers",
			object: map[string]any{
				"kind": "StatefulSet",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"initContainers": []any{
								map[string]any{"name": "init-container-1"},
							},
						},
					},
				},
			},
			expected:    []string{"init-container-1"},
			expectError: false,
		},
		{
			name: "ReplicaSet with no containers",
			object: map[string]any{
				"kind": "ReplicaSet",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{},
					},
				},
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "Invalid kind",
			object: map[string]any{
				"kind": "UnknownKind",
			},
			expected:    nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: tt.object,
				},
			}

			allContainers, err := obj.GetAllContainers()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				var containerNames []string
				for _, container := range allContainers {
					containerNames = append(containerNames, container.Name)
				}
				assert.Equal(t, tt.expected, containerNames)
			}
		})
	}
}

func TestStoreObject_GetPodSecurityContext(t *testing.T) {
	tests := []struct {
		name        string
		object      map[string]any
		expected    *v1.PodSecurityContext
		expectError bool
	}{
		{
			name: "Deployment with PodSecurityContext",
			object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"securityContext": map[string]any{
								"runAsUser": int64(1000),
							},
						},
					},
				},
			},
			expected:    &v1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1000)},
			expectError: false,
		},
		{
			name: "Pod without PodSecurityContext",
			object: map[string]any{
				"kind": "Pod",
				"spec": map[string]any{},
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "StatefulSet with PodSecurityContext",
			object: map[string]any{
				"kind": "StatefulSet",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"securityContext": map[string]any{
								"fsGroup": int64(2000),
							},
						},
					},
				},
			},
			expected:    &v1.PodSecurityContext{FSGroup: func(i int64) *int64 { return &i }(2000)},
			expectError: false,
		},
		{
			name: "Invalid kind",
			object: map[string]any{
				"kind": "UnknownKind",
			},
			expected:    nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: tt.object,
				},
			}

			securityContext, err := obj.GetPodSecurityContext()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, securityContext)
			}
		})
	}
}

func TestStoreObject_IsHostNetwork(t *testing.T) {
	tests := []struct {
		name        string
		object      map[string]any
		expected    bool
		expectError bool
	}{
		{
			name: "Deployment with HostNetwork enabled",
			object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"hostNetwork": true,
						},
					},
				},
			},
			expected:    true,
			expectError: false,
		},
		{
			name: "Pod without HostNetwork",
			object: map[string]any{
				"kind": "Pod",
				"spec": map[string]any{
					"hostNetwork": false,
				},
			},
			expected:    false,
			expectError: false,
		},
		{
			name: "StatefulSet with HostNetwork enabled",
			object: map[string]any{
				"kind": "StatefulSet",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"hostNetwork": true,
						},
					},
				},
			},
			expected:    true,
			expectError: false,
		},
		{
			name: "ReplicaSet without HostNetwork",
			object: map[string]any{
				"kind": "ReplicaSet",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"hostNetwork": false,
						},
					},
				},
			},
			expected:    false,
			expectError: false,
		},
		{
			name: "Invalid kind",
			object: map[string]any{
				"kind": "UnknownKind",
			},
			expected:    false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := StoreObject{
				Unstructured: unstructured.Unstructured{
					Object: tt.object,
				},
			}

			hostNetwork, err := obj.IsHostNetwork()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, hostNetwork)
			}
		})
	}
}
