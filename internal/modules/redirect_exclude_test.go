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

package modules

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
)

// TestRemapListenerSetRedirectExcludeRules proves that a module's .dmtlint.yaml
// exclude-rules for listenerset-redirect (parsed into
// config.ListenerSetRedirectExcludeList by viper/mapstructure, keyed by the
// `name`/`section` fields) actually reach the pkg.ListenerSetRedirectExclude the
// rule itself reads from — including the empty-ListenerSet ("any set") wildcard.
func TestRemapListenerSetRedirectExcludeRules(t *testing.T) {
	configSettings := &config.LintersSettings{
		Templates: config.TemplatesSettings{
			ExcludeRules: config.TemplatesExcludeRules{
				ListenerSetRedirect: config.ListenerSetRedirectExcludeList{
					{ListenerSet: "istio", Section: "istio-metadata"},
					{Section: "any-set-wildcard"},
				},
			},
		},
	}

	settings := remapLinterSettings(configSettings, &global.Linters{})

	require.Equal(t, pkg.ListenerSetRedirectExcludeList{
		{ListenerSet: "istio", Section: "istio-metadata"},
		{ListenerSet: "", Section: "any-set-wildcard"},
	}, settings.Templates.ExcludeRules.ListenerSetRedirect)
}

// TestRemapHTTPRouteRedirectExcludeRules proves the same remap path for
// httproute-redirect excludes.
func TestRemapHTTPRouteRedirectExcludeRules(t *testing.T) {
	configSettings := &config.LintersSettings{
		Templates: config.TemplatesSettings{
			ExcludeRules: config.TemplatesExcludeRules{
				HTTPRouteRedirect: config.HTTPRouteRedirectExcludeList{
					{ListenerSet: "istio", Section: "istio-redirect"},
					{Section: "any-set-wildcard"},
				},
			},
		},
	}

	settings := remapLinterSettings(configSettings, &global.Linters{})

	require.Equal(t, pkg.HTTPRouteRedirectExcludeList{
		{ListenerSet: "istio", Section: "istio-redirect"},
		{ListenerSet: "", Section: "any-set-wildcard"},
	}, settings.Templates.ExcludeRules.HTTPRouteRedirect)
}

// TestRemapListenerSetRedirectRuleLevel proves the rule-level impact override
// reaches pkg.TemplatesLinterRules, mirroring the other Templates rules.
func TestRemapListenerSetRedirectRuleLevel(t *testing.T) {
	settings := remapLinterSettings(&config.LintersSettings{}, &global.Linters{
		Templates: global.TemplatesLinterConfig{
			Rules: global.TemplatesLinterRules{
				ListenerSetRedirectRule: global.RuleConfig{Impact: pkg.Warn.String()},
			},
		},
	})

	require.Equal(t, pkg.Warn, *settings.Templates.Rules.ListenerSetRedirectRule.GetLevel())
}

// TestRemapHTTPRouteRedirectRuleLevel proves the same for httproute-redirect.
func TestRemapHTTPRouteRedirectRuleLevel(t *testing.T) {
	settings := remapLinterSettings(&config.LintersSettings{}, &global.Linters{
		Templates: global.TemplatesLinterConfig{
			Rules: global.TemplatesLinterRules{
				HTTPRouteRedirectRule: global.RuleConfig{Impact: pkg.Warn.String()},
			},
		},
	})

	require.Equal(t, pkg.Warn, *settings.Templates.Rules.HTTPRouteRedirectRule.GetLevel())
}
