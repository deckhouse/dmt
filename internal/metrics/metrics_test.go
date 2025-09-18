package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

func Test_SetLinterWarningsMetrics_AddsWarningsForAllLinters(t *testing.T) {
	metrics = nil
	metrics = GetClient(".")
	cfg := global.Global{
		Linters: global.Linters{
			Container:  global.ContainerLinterConfig{},
			Hooks:      global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Images:     global.ImagesLinterConfig{},
			License:    global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Module:     global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			NoCyrillic: global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			OpenAPI:    global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Rbac:       global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Templates:  global.LinterConfig{Impact: ptr.To(pkg.Warn)},
		},
	}
	cfg.Linters.Container.Impact = ptr.To(pkg.Warn)
	cfg.Linters.Images.Impact = ptr.To(pkg.Warn)

	SetLinterWarningsMetrics(cfg)
	num, err := testutil.GatherAndCount(metrics.Gatherer, "dmt_linter_info")
	require.NoError(t, err)
	require.Equal(t, 9, num)
}

func Test_SetLinterWarningsMetrics_NoWarningsWhenNoLinters(t *testing.T) {
	metrics = nil
	metrics = GetClient(".")
	cfg := global.Global{
		Linters: global.Linters{},
	}
	SetLinterWarningsMetrics(cfg)
	num, err := testutil.GatherAndCount(metrics.Gatherer, "dmt_linter_info")
	require.NoError(t, err)
	require.Equal(t, 0, num)
}

func Test_SetLinterWarningsMetrics_AddsWarningsForSpecificLinters(t *testing.T) {
	metrics = nil
	metrics = GetClient(".")
	cfg := global.Global{
		Linters: global.Linters{
			Container: global.ContainerLinterConfig{},
			Hooks:     global.LinterConfig{Impact: nil},
		},
	}

	cfg.Linters.Container.Impact = ptr.To(pkg.Warn)

	SetLinterWarningsMetrics(cfg)
	num, err := testutil.GatherAndCount(metrics.Gatherer, "dmt_linter_info")
	require.NoError(t, err)
	require.Equal(t, 1, num)
}
