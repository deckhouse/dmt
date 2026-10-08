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
	assert.Equal(t, SystemLevels, LevelsOf("networking"), "a subsystem carries the system levels")
	assert.Nil(t, LevelsOf("tenant"))

	// The lineages are separate slices: reordering one must not reorder another.
	swapFirstTwo(ProjectLevels)
	defer swapFirstTwo(ProjectLevels)

	assert.Equal(t, "viewer", NamespaceLevels[0])
}

func swapFirstTwo(levels []string) { levels[0], levels[1] = levels[1], levels[0] }

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

func TestVerbsAndKinds(t *testing.T) {
	assert.Equal(t, append(append([]string{}, ResourceVerbs...), "*"), Verbs)
	assert.True(t, IsLegacyKind(KindLegacyUse))
	assert.True(t, IsLegacyKind(KindLegacyManage))
	assert.False(t, IsLegacyKind(KindCapability))
	assert.True(t, IsSubsystem("security"))
	assert.False(t, IsSubsystem("tenant"))
}
