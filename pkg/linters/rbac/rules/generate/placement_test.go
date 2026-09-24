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

package generate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

func placementDecl() *rbacyaml.Declaration {
	return &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		Resources: []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Namespaced", Namespace: map[string][]string{"viewer": {"get"}}}},
	}
}

// Every module namespace carries the label user-authz projects the use roles by, kube-system
// included; default does not (regression hunt, B2).
func TestBuild_NamespaceLabelOutsideD8(t *testing.T) {
	for ns, want := range map[string]bool{"d8-m": true, "kube-system": true, "default": false} {
		model, err := Build(Input{Module: "m", Namespace: ns, Subsystems: []string{"security"}, Decl: placementDecl()})
		require.NoError(t, err)

		labelled := false

		for _, f := range model.Files {
			for _, o := range f.Objects {
				if o.Labels[rbaccontract.LabelNamespace] == ns {
					labelled = true
				}
			}
		}

		assert.Equal(t, want, labelled, ns)
	}
}

// A namespace access entry in a component directory gets the name the placement rule wants there
// (regression hunt, B8).
func TestBuild_AccessNamesFollowThePlacementRule(t *testing.T) {
	decl := placementDecl()
	rules := []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}
	subjects := []rbacyaml.Subject{{Kind: "Group", Name: "g"}}
	decl.Access = []rbacyaml.Access{
		{Name: "reader", Subjects: subjects, NamespaceRules: rules},
		{Name: "reader", Path: "webhook/tls", Subjects: subjects, NamespaceRules: rules},
	}
	// Two entries with one name are a duplicate for the validator, not for the generator.
	model, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	root := model.File("templates/rbac-to-us.yaml")
	require.NotNil(t, root)
	assert.Equal(t, "access-to-m-reader", root.Objects[0].Name)

	nested := model.File("templates/webhook/tls/rbac-to-us.yaml")
	require.NotNil(t, nested)
	assert.Equal(t, "access-to-webhook-tls-reader", nested.Objects[0].Name)
}

// Account annotations land on the ServiceAccount, rbacAnnotations on its roles and bindings
// (regression hunt, B7).
func TestBuild_AccountAnnotations(t *testing.T) {
	decl := placementDecl()
	decl.ServiceAccounts = []rbacyaml.ServiceAccount{{
		Name:            "m",
		Annotations:     map[string]string{"helm.sh/resource-policy": "keep"},
		RBACAnnotations: map[string]string{"werf.io/deploy-on": "pre-install"},
		ClusterRules:    []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}},
	}}

	model, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	file := model.File("templates/rbac-for-us.yaml")
	require.NotNil(t, file)

	for _, o := range file.Objects {
		if o.Kind == "ServiceAccount" {
			assert.Equal(t, map[string]string{"helm.sh/resource-policy": "keep"}, o.Annotations)
		} else {
			assert.Equal(t, map[string]string{"werf.io/deploy-on": "pre-install"}, o.Annotations, o.Identity())
		}
	}

	rendered := RenderFile(*file)
	assert.Equal(t, 1, strings.Count(rendered, `helm.sh/resource-policy: "keep"`))
	assert.Equal(t, 2, strings.Count(rendered, `werf.io/deploy-on: "pre-install"`))
}

// Access and Prometheus entries carry labels and annotations onto their role and binding
// (regression hunt 2, A2).
func TestBuild_AccessMetadata(t *testing.T) {
	decl := placementDecl()
	decl.Access = []rbacyaml.Access{{
		Name: "manager", Subjects: []rbacyaml.Subject{{Kind: "Group", Name: "g"}},
		Labels: map[string]string{"app": "capi"}, Annotations: map[string]string{"werf.io/deploy-on": "pre-install"},
		ClusterRules: []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}},
	}}
	decl.PrometheusAccess = &rbacyaml.PrometheusAccess{Deployments: []string{"m"}, Labels: map[string]string{"app": "m"}}

	model, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	for _, o := range model.File("templates/rbac-for-us.yaml").Objects {
		assert.Equal(t, map[string]string{"app": "capi"}, o.Labels, o.Identity())
		assert.Equal(t, map[string]string{"werf.io/deploy-on": "pre-install"}, o.Annotations, o.Identity())
	}

	for _, o := range model.File("templates/rbac-to-us.yaml").Objects {
		assert.Equal(t, map[string]string{"app": "m"}, o.Labels, o.Identity())
	}
}
