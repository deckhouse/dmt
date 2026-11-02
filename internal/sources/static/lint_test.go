package static

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/pkg/errors"
)

// TestTargetsFailsWhenGlobalValuesAreUnreadable reproduces deckhouse linted outside
// /deckhouse: the $ref in global-hooks/openapi/values.yaml points at a file that is
// not there, no module can be built, and the run must fail instead of passing empty.
func TestTargetsFailsWhenGlobalValuesAreUnreadable(t *testing.T) {
	root := t.TempDir()
	openapi := filepath.Join(root, "global-hooks", "openapi")
	require.NoError(t, os.MkdirAll(openapi, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "modules"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(openapi, "config-values.yaml"), []byte("type: object\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(openapi, "values.yaml"), []byte(`type: object
properties:
  clusterConfiguration:
    $ref: '/nonexistent/candi/openapi/cluster_configuration.yaml#/apiVersions/0/openAPISpec'
`), 0o644))

	err := NewSource(root).Targets(t.Context(), nil, errors.NewLintRuleErrorsList(), func(manager.Target) bool {
		t.Fatal("no module may be linted without global values")

		return false
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get global values")
	assert.Contains(t, err.Error(), "cluster_configuration.yaml")
}

// TestMergeValuesLeavesBaseAlone is the guard for the invariant every matrix render
// depends on: the --values-file tree is shared by all of them, so a variant that
// wrote into it would hand its own overrides to every variant rendered afterwards
// and each one would be linted under the union of the combinations before it.
func TestMergeValuesLeavesBaseAlone(t *testing.T) {
	base := chartutil.Values{
		"mod": map[string]any{
			"enabled": false,
			"nested":  map[string]any{"mode": "Direct"},
			"list":    []any{"a"},
		},
	}

	first := mergeValues(base, chartutil.Values{"mod": map[string]any{"enabled": true}})
	assert.Equal(t, true, first["mod"].(map[string]any)["enabled"])

	second := mergeValues(base, chartutil.Values{
		"mod": map[string]any{"nested": map[string]any{"mode": "Proxy"}},
	})

	assert.Equal(t, false, base["mod"].(map[string]any)["enabled"],
		"the first variant's override must not have reached the shared base")
	assert.Equal(t, "Direct", base["mod"].(map[string]any)["nested"].(map[string]any)["mode"],
		"the second variant's override must not have reached the shared base")
	assert.Equal(t, false, second["mod"].(map[string]any)["enabled"],
		"a variant must render with its own overrides only, not those of earlier ones")
}

// TestMergeValuesWithoutOverridesSharesTheBase pins the plain (non-matrix) path: the
// single render of a module is handed the values tree as it is, with no copying.
func TestMergeValuesWithoutOverridesSharesTheBase(t *testing.T) {
	base := chartutil.Values{"mod": map[string]any{"enabled": true}}

	assert.Equal(t, base, mergeValues(base, nil))
	assert.Nil(t, mergeValues(nil, nil))
}
