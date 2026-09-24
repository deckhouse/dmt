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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadFrom(t *testing.T, content string) error {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".dmtlint.yaml"), []byte(content), 0o600))

	l := NewLoader(&RootConfig{}, "")
	l.viper = viper.New()
	l.viper.SetConfigType("yaml")
	l.viper.SetConfigName(".dmtlint")
	l.viper.AddConfigPath(dir)

	return l.Load()
}

// The rbac blocks refuse unknown keys: a misspelled per-rule level or exclusion must not be
// dropped in silence.
func TestLoader_RbacKeysAreStrict(t *testing.T) {
	t.Run("known keys load", func(t *testing.T) {
		require.NoError(t, loadFrom(t, `
global:
  linters-settings:
    rbac:
      impact: error
      rules:
        coverage: {impact: warn}
        sync: {impact: warn}
        contract: {impact: warn}
linters-settings:
  rbac:
    impact: error
    exclude-rules:
      coverage: [deckhouse.io/internals]
      contract:
        - kind: ClusterRole
          name: d8:namespace-capability:x:view
      sync: []
`))
	})

	t.Run("a misspelled rule under the root block is an error", func(t *testing.T) {
		err := loadFrom(t, "global:\n  linters-settings:\n    rbac:\n      rules:\n        coverge: {impact: warn}\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown key(s) coverge under "global.linters-settings.rbac.rules"`)
		assert.Contains(t, err.Error(), "the accepted keys are contract, coverage, sync")
	})

	t.Run("per-rule levels do not belong to the module block", func(t *testing.T) {
		// ADR: per-rule levels are read from the root configuration only, as for every dmt linter.
		err := loadFrom(t, "linters-settings:\n  rbac:\n    rules:\n      coverage: {impact: warn}\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown key(s) rules under "linters-settings.rbac"`)
	})

	t.Run("an unknown level is an error, not a silent error level", func(t *testing.T) {
		err := loadFrom(t, "global:\n  linters-settings:\n    rbac:\n      rules:\n        sync: {impact: ignore}\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `global.linters-settings.rbac.rules.sync.impact is ignore: the levels are ignored, warn, error, critical`)
	})

	t.Run("an unknown exclusion key is an error", func(t *testing.T) {
		err := loadFrom(t, "linters-settings:\n  rbac:\n    exclude-rules:\n      coverage-rule: [x]\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown key(s) coverage-rule under "linters-settings.rbac.exclude-rules"`)
	})

	t.Run("a misspelled impact of one rule is an error", func(t *testing.T) {
		err := loadFrom(t, "global:\n  linters-settings:\n    rbac:\n      rules:\n        coverage: {impakt: warn}\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown key(s) impakt under "global.linters-settings.rbac.rules.coverage": the accepted keys are impact`)
	})

	t.Run("a misspelled key of an exclusion entry is an error", func(t *testing.T) {
		err := loadFrom(t, "linters-settings:\n  rbac:\n    exclude-rules:\n      contract:\n        - kidn: ClusterRole\n          name: x\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown key(s) kidn under "linters-settings.rbac.exclude-rules.contract[0]": the accepted keys are kind, name`)
	})

	t.Run("every problem is reported at once", func(t *testing.T) {
		err := loadFrom(t, "global:\n  linters-settings:\n    rbac:\n      rules:\n        coverge: {impact: warn}\nlinters-settings:\n  rbac:\n    exclude-rules:\n      sync: [just-a-string]\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown key(s) coverge")
		assert.Contains(t, err.Error(), `entry 0 under "linters-settings.rbac.exclude-rules.sync" is not a kind/name pair`)
	})

	t.Run("other linters keep the lenient behaviour", func(t *testing.T) {
		require.NoError(t, loadFrom(t, "linters-settings:\n  container:\n    impakt: warn\n"))
	})
}
