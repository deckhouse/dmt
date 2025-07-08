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
			Container:  global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Hooks:      global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Images:     global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			License:    global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Module:     global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			NoCyrillic: global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			OpenAPI:    global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Rbac:       global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Templates:  global.LinterConfig{Impact: ptr.To(pkg.Warn)},
		},
	}
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
			Container: global.LinterConfig{Impact: ptr.To(pkg.Warn)},
			Hooks:     global.LinterConfig{Impact: nil},
		},
	}
	SetLinterWarningsMetrics(cfg)
	num, err := testutil.GatherAndCount(metrics.Gatherer, "dmt_linter_info")
	require.NoError(t, err)
	require.Equal(t, 1, num)
}
