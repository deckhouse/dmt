package module

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
)

func TestRemapOpenAPIBilingualRuleLevel(t *testing.T) {
	t.Run("defaults to error", func(t *testing.T) {
		settings := remapLinterSettings(&config.LintersSettings{}, &global.Linters{})

		require.Equal(t, pkg.Error, *settings.OpenAPI.Rules.BilingualRule.GetLevel())
	})

	t.Run("uses global warning level", func(t *testing.T) {
		settings := remapLinterSettings(&config.LintersSettings{}, &global.Linters{
			OpenAPI: global.OpenAPILinterConfig{
				Rules: global.OpenAPIRules{
					BilingualRule: global.RuleConfig{Impact: pkg.Warn.String()},
				},
			},
		})

		require.Equal(t, pkg.Warn, *settings.OpenAPI.Rules.BilingualRule.GetLevel())
	})
}
