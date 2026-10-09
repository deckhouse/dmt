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

package rbaccontract

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLegacyKebab(t *testing.T) {
	for level, want := range map[string]string{
		"User":           "user",
		"PrivilegedUser": "privileged-user",
		"Editor":         "editor",
		"Admin":          "admin",
		"ClusterEditor":  "cluster-editor",
		"ClusterAdmin":   "cluster-admin",
		"SuperAdmin":     "super-admin",
	} {
		assert.Equal(t, want, LegacyKebab(level), level)
	}
}

func TestLevelsOf(t *testing.T) {
	assert.Equal(t, NamespaceLevels, LevelsOf(LineageNamespace))
	assert.Equal(t, NamespaceLevels, LevelsOf(LineageProject))
	assert.Equal(t, SystemLevels, LevelsOf(LineageSystem))
	assert.Equal(t, SystemLevels, LevelsOf("network"), "a subsystem carries the system levels")
	assert.Equal(t, SystemLevels, LevelsOf("managed-services"), "a subsystem id may hold a dash")
	assert.Nil(t, LevelsOf("networking"), "a subsystem of the legacy scheme is no lineage")
	assert.Nil(t, LevelsOf("tenant"))

	// The lineages are separate slices: reordering one must not reorder another.
	swapFirstTwo(ProjectLevels)
	defer swapFirstTwo(ProjectLevels)

	assert.Equal(t, "viewer", NamespaceLevels[0])
}

func swapFirstTwo(levels []string) { levels[0], levels[1] = levels[1], levels[0] }

// The subsystems are the eight of DKP; those of the legacy scheme went into cluster and network.
func TestIsSubsystem(t *testing.T) {
	for _, name := range []string{"iam", "security", "cluster", "delivery", "network", "storage", "observability", "managed-services"} {
		assert.True(t, IsSubsystem(name), name)
	}

	for _, name := range []string{"deckhouse", "infrastructure", "kubernetes", "networking", "managed-service", "tenant"} {
		assert.False(t, IsSubsystem(name), name)
	}
}

func TestReplacementOf(t *testing.T) {
	for legacy, want := range map[string]string{"deckhouse": "cluster", "infrastructure": "cluster", "kubernetes": "cluster", "networking": "network"} {
		got, ok := ReplacementOf(legacy)
		assert.True(t, ok, legacy)
		assert.Equal(t, want, got, legacy)
		assert.True(t, IsSubsystem(got), "the replacement of %s is a subsystem of the role model", legacy)
	}

	for _, name := range append(slices.Clone(Subsystems), "virtualization", "") {
		_, ok := ReplacementOf(name)
		assert.False(t, ok, name)
	}
}

func TestCapabilityActionRoundTrip(t *testing.T) {
	for _, level := range NamespaceLevels {
		assert.Equal(t, level, LevelOfAction(CapabilityAction(level)), level)
	}

	assert.Equal(t, "view", CapabilityAction("viewer"))
	assert.Equal(t, "edit", CapabilityAction("manager"))
	assert.True(t, IsConventionalAction("view"))
	assert.False(t, IsConventionalAction("admin"))
}

func TestBindingSuffix(t *testing.T) {
	assert.Equal(t, "rbac-proxy", BindingSuffix("d8:rbac-proxy"))
	assert.Equal(t, "system-auth-delegator", BindingSuffix("system:auth-delegator"))
	assert.Equal(t, "cluster-admin", BindingSuffix("cluster-admin"))
}

// The name of a foreign-namespace binding follows the placement rule, and a role named after the
// account lends it the rest of its name instead of the module and the account twice (dry run on
// security-events-manager, 2026-10-09).
func TestAccountForeignBindingName(t *testing.T) {
	for name, tc := range map[string]struct {
		module, path, account, ns, role, want string
	}{
		"kube-system, a role of the platform": {
			module: "cert-manager", path: "cainjector", account: "cainjector", ns: "kube-system",
			role: "extension-apiserver-authentication-reader",
			want: "d8:cert-manager:cainjector:extension-apiserver-authentication-reader",
		},
		"kube-system, a role named after the account": {
			module: "m", path: "a", account: "a", ns: "kube-system", role: "d8:m:a:x", want: "d8:m:a:x",
		},
		"platform namespace, a role named after the account with d8:": {
			module: "security-events-manager", path: "controller", account: "controller", ns: "d8-log-shipper",
			role: "d8:security-events-manager:controller:log-shipper-secrets",
			want: "security-events-manager:controller:log-shipper-secrets",
		},
		"platform namespace, a role named after the account": {
			module: "m", path: "a", account: "a", ns: "d8-monitoring", role: "m:a:x", want: "m:a:x",
		},
		"platform namespace, another role": {
			module: "m", path: "a/b", account: "m-a-b", ns: "d8-system", role: "some-role", want: "m:a:b:some-role",
		},
		"root account, kube-system": {
			module: "m", account: "acct", ns: "kube-system", role: "d8:m:acct:x", want: "d8:m:acct:x",
		},
		"another namespace keeps the d8: prefix": {
			module: "m", path: "a", account: "a", ns: "d8-other", role: "d8:m:a:x", want: "d8:m:a:x",
		},
		"the prefix alone is no name to lend": {
			module: "m", path: "a", account: "a", ns: "kube-system", role: "d8:m:a:", want: "d8:m:a:m-a-",
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, AccountForeignBindingName(tc.module, tc.path, tc.account, tc.ns, tc.role))
		})
	}
}

func TestVerbsAndKinds(t *testing.T) {
	assert.Equal(t, append(append([]string{}, ResourceVerbs...), "*"), Verbs)
	assert.True(t, IsLegacyKind(KindLegacyUse))
	assert.True(t, IsLegacyKind(KindLegacyManage))
	assert.False(t, IsLegacyKind(KindCapability))
}
