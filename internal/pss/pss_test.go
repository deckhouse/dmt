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

package pss

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecks(t *testing.T) {
	templates, err := renderConstraintTemplates()
	require.NoError(t, err)
	assert.Len(t, templates, 13)

	checks, err := Checks()
	require.NoError(t, err)

	byStd := map[string]map[string]Check{Baseline: {}, Restricted: {}}
	for _, c := range checks {
		require.NotContains(t, byStd[c.Standard], c.Kind, "duplicate kind within %s", c.Standard)
		byStd[c.Standard][c.Kind] = c

		assert.NotEmpty(t, c.Rego, c.Kind)
		assert.NotEmpty(t, c.Libs, c.Kind)
		_, err := ast.ParseModule(c.Kind+".rego", c.Rego)
		require.NoError(t, err, c.Kind)

		for i, lib := range c.Libs {
			_, err := ast.ParseModule(c.Kind+".lib", lib)
			require.NoError(t, err, "%s lib %d", c.Kind, i)
		}
	}

	assert.Len(t, byStd[Baseline], 10)
	assert.Len(t, byStd[Restricted], 5)

	assert.Equal(t, map[string]any{"allowHostNetwork": false, "ranges": []any{}}, byStd[Baseline]["D8HostNetwork"].Parameters)
	require.Contains(t, byStd[Baseline], "D8PrivilegedContainer")
	assert.Empty(t, byStd[Baseline]["D8PrivilegedContainer"].Parameters)

	for _, kind := range []string{"D8AllowedCapabilities", "D8AllowedSeccompProfiles"} {
		require.Contains(t, byStd[Baseline], kind)
		require.Contains(t, byStd[Restricted], kind)
		assert.NotEqual(t, byStd[Baseline][kind].Parameters, byStd[Restricted][kind].Parameters, kind)
	}
}
