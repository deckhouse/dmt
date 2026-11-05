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

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	testModuleName      = "security-events-manager"
	testModuleNamespace = "d8-security-events-manager"
)

// placementObject describes one rendered manifest as the object store sees it:
// a chart-relative template path plus the object's identity.
type placementObject struct {
	shortPath string
	kind      string
	name      string
	namespace string
}

func placementStorage(t *testing.T, objects ...placementObject) map[storage.ResourceIndex]storage.StoreObject {
	t.Helper()

	store := storage.NewUnstructuredObjectStore()

	for _, o := range objects {
		metadata := map[string]any{"name": o.name}
		if o.namespace != "" {
			metadata["namespace"] = o.namespace
		}

		content := map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       o.kind,
			"metadata":   metadata,
		}

		require.NoError(t, store.Put("/module/"+o.shortPath, o.shortPath, content, []byte(o.shortPath+o.name)))
	}

	return store.Storage
}

func runPlacementRule(t *testing.T, objects ...placementObject) []pkg.LinterError {
	t.Helper()

	mc := minimock.NewController(t)

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(placementStorage(t, objects...))
	mod.GetNameMock.Optional().Return(testModuleName)
	mod.GetNamespaceMock.Optional().Return(testModuleNamespace)

	errorList := errors.NewLintRuleErrorsList()
	NewPlacementRule(nil, mod, errorList).Check(t.Context())

	return errorList.GetErrors()
}

// Regression: RBACv2Path ("templates/rbac") used to be matched with a bare
// strings.HasPrefix, which also swallowed the root "templates/rbac-for-us.yaml"
// and "templates/rbac-to-us.yaml" — so a module keeping all of its RBAC at the
// root was never checked at all.
func TestPlacementRule_RootRBACFilesAreChecked(t *testing.T) {
	tests := []struct {
		name    string
		object  placementObject
		wantMsg string
	}{
		{
			name: "ServiceAccount in root rbac-for-us.yaml",
			object: placementObject{
				shortPath: RootRBACForUsPath,
				kind:      "ServiceAccount",
				name:      "wrong-name",
				namespace: testModuleNamespace,
			},
			wantMsg: `Name of ServiceAccount in "templates/rbac-for-us.yaml" should be equal to Chart Name (security-events-manager)`,
		},
		{
			name: "ClusterRole in root rbac-for-us.yaml",
			object: placementObject{
				shortPath: RootRBACForUsPath,
				kind:      "ClusterRole",
				name:      "wrong-name",
			},
			wantMsg: `Name of ClusterRole in "templates/rbac-for-us.yaml" should start with "d8:security-events-manager"`,
		},
		{
			name: "RoleBinding in root rbac-to-us.yaml",
			object: placementObject{
				shortPath: RootRBACToUsPath,
				kind:      "RoleBinding",
				name:      "wrong-name",
				namespace: testModuleNamespace,
			},
			wantMsg: `RoleBinding in "templates/rbac-to-us.yaml" should start with "access-to-security-events-manager"`,
		},
		{
			name: "unexpected kind in root rbac-for-us.yaml",
			object: placementObject{
				shortPath: RootRBACForUsPath,
				kind:      "ConfigMap",
				name:      "some-config",
				namespace: testModuleNamespace,
			},
			wantMsg: "kind ConfigMap not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lintErrors := runPlacementRule(t, tt.object)

			require.Len(t, lintErrors, 1)
			assert.Equal(t, tt.wantMsg, lintErrors[0].Text)
			assert.Equal(t, tt.object.shortPath, lintErrors[0].FilePath)
		})
	}
}

func TestPlacementRule_RootRBACFilesAccepted(t *testing.T) {
	lintErrors := runPlacementRule(t,
		placementObject{
			shortPath: RootRBACForUsPath,
			kind:      "ServiceAccount",
			name:      testModuleName,
			namespace: testModuleNamespace,
		},
		placementObject{
			shortPath: RootRBACForUsPath,
			kind:      "ClusterRole",
			name:      "d8:" + testModuleName + ":rbac-proxy",
		},
		placementObject{
			shortPath: RootRBACToUsPath,
			kind:      "RoleBinding",
			name:      "access-to-" + testModuleName,
			namespace: testModuleNamespace,
		},
	)

	assert.Empty(t, lintErrors)
}

// Objects under the RBAC v2 directory are validated elsewhere and must stay skipped.
func TestPlacementRule_RBACv2DirectorySkipped(t *testing.T) {
	lintErrors := runPlacementRule(t,
		placementObject{
			shortPath: RBACv2Path + "/module.yaml",
			kind:      "ServiceAccount",
			name:      "totally-wrong",
			namespace: "kube-system",
		},
		placementObject{
			shortPath: RBACv2Path + "/nested/roles.yaml",
			kind:      "ClusterRole",
			name:      "totally-wrong-cluster-role",
		},
		placementObject{
			shortPath: UserAuthzClusterRolePath,
			kind:      "ClusterRole",
			name:      "totally-wrong-user-authz-role",
		},
	)

	assert.Empty(t, lintErrors)
}

func TestPlacementRule_NestedRBACFiles(t *testing.T) {
	lintErrors := runPlacementRule(t,
		placementObject{
			shortPath: "templates/collector/rbac-for-us.yaml",
			kind:      "ServiceAccount",
			name:      "collector",
			namespace: testModuleNamespace,
		},
		placementObject{
			shortPath: "templates/parser/rbac-for-us.yaml",
			kind:      "ServiceAccount",
			name:      "wrong-name",
			namespace: testModuleNamespace,
		},
	)

	require.Len(t, lintErrors, 1)
	assert.Equal(t,
		`Name of ServiceAccount should be equal to "parser" or "security-events-manager-parser"`,
		lintErrors[0].Text)
}
