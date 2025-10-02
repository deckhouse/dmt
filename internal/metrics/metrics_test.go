package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

func Test_SetLinterWarningsMetrics_AddsWarningsForAllLinters(t *testing.T) {
	metrics = nil
	metrics = GetClient(".")
	cfg := &global.Global{
		Linters: global.Linters{
			Container:     global.ContainerLinterConfig{},
			Hooks:         global.LinterConfig{Impact: pkg.Warn.String()},
			Images:        global.ImagesLinterConfig{},
			License:       global.LinterConfig{Impact: pkg.Warn.String()},
			Module:        global.ModuleLinterConfig{},
			NoCyrillic:    global.LinterConfig{Impact: pkg.Warn.String()},
			OpenAPI:       global.LinterConfig{Impact: pkg.Warn.String()},
			Rbac:          global.LinterConfig{Impact: pkg.Warn.String()},
			Templates:     global.LinterConfig{Impact: pkg.Warn.String()},
			Documentation: global.DocumentationLinterConfig{},
		},
	}

	cfg.Linters.Container.Impact = pkg.Warn.String()
	cfg.Linters.Images.Impact = pkg.Warn.String()
	cfg.Linters.Module.Impact = pkg.Warn.String()
	cfg.Linters.Documentation.Impact = pkg.Warn.String()

	SetLinterWarningsMetrics(cfg)
	num, err := testutil.GatherAndCount(metrics.Gatherer, "dmt_linter_info")
	require.NoError(t, err)
	require.Equal(t, 10, num)
}

func Test_SetLinterWarningsMetrics_NoWarningsWhenNoLinters(t *testing.T) {
	metrics = nil
	metrics = GetClient(".")
	cfg := &global.Global{
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
	cfg := &global.Global{
		Linters: global.Linters{
			Container: global.ContainerLinterConfig{},
			Hooks:     global.LinterConfig{},
		},
	}

	cfg.Linters.Container.Impact = pkg.Warn.String()

	SetLinterWarningsMetrics(cfg)
	num, err := testutil.GatherAndCount(metrics.Gatherer, "dmt_linter_info")
	require.NoError(t, err)
	require.Equal(t, 1, num)
}
