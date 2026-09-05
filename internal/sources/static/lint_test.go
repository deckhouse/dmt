package static

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chartutil"
)

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
