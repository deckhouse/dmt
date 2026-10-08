package modules

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
)

func TestRemapSecurityPolicyExceptionRuleLevels(t *testing.T) {
	t.Run("description defaults to error, unused to warn", func(t *testing.T) {
		settings := remapLinterSettings(&config.LintersSettings{}, &global.Linters{})

		require.Equal(t, pkg.Error, *settings.Container.Rules.SPEDescriptionRule.GetLevel())
		require.Equal(t, pkg.Error, *settings.Container.Rules.SPESchemaRule.GetLevel())
		require.Equal(t, pkg.Warn, *settings.Container.Rules.SPEUnusedRule.GetLevel())
	})

	t.Run("configured separately", func(t *testing.T) {
		settings := remapLinterSettings(&config.LintersSettings{}, &global.Linters{
			Container: global.ContainerLinterConfig{
				Rules: global.ContainerRules{
					SPEDescriptionRule: global.RuleConfig{Impact: pkg.Warn.String()},
					SPEUnusedRule:      global.RuleConfig{Impact: pkg.Error.String()},
					SPESchemaRule:      global.RuleConfig{Impact: pkg.Warn.String()},
				},
			},
		})

		require.Equal(t, pkg.Warn, *settings.Container.Rules.SPEDescriptionRule.GetLevel())
		require.Equal(t, pkg.Error, *settings.Container.Rules.SPEUnusedRule.GetLevel())
		require.Equal(t, pkg.Warn, *settings.Container.Rules.SPESchemaRule.GetLevel())
	})
}

func TestRemapSecurityPolicyExceptionUnusedExcludes(t *testing.T) {
	settings := remapLinterSettings(&config.LintersSettings{
		Container: config.ContainerSettings{
			ExcludeRules: config.ContainerExcludeRules{
				SecurityPolicyExceptionUnused: config.StringRuleExcludeList{"d8-control-plane-manager"},
			},
		},
	}, &global.Linters{})

	require.Equal(t, []pkg.StringRuleExclude{"d8-control-plane-manager"},
		settings.Container.ExcludeRules.SecurityPolicyExceptionUnused.Get())
}
