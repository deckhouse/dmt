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

// TestRemapHTTPSCertificateReuseExcludeRules proves that a module's
// .dmtlint.yaml exclude-rules for https-certificate-reuse (parsed into
// config.TemplatesExcludeRules by viper/mapstructure) actually reach the
// pkg.TemplatesExcludeRules the rule itself reads from — the same path
// IngressEnablement/GatewayEnablement already use.
func TestRemapHTTPSCertificateReuseExcludeRules(t *testing.T) {
	configSettings := &config.LintersSettings{
		Templates: config.TemplatesSettings{
			ExcludeRules: config.TemplatesExcludeRules{
				HTTPSCertificateReuse: config.PathRuleExclude{
					Files:       config.StringRuleExcludeList{"templates/legacy-certificate.yaml"},
					Directories: config.DirectoryRuleExcludeList{"templates/vendor/"},
				},
			},
		},
	}

	settings := remapLinterSettings(configSettings, &global.Linters{})

	excludes := settings.Templates.ExcludeRules.HTTPSCertificateReuse
	require.Equal(t, pkg.StringRuleExcludeList{"templates/legacy-certificate.yaml"}, excludes.Files)
	require.Equal(t, pkg.DirectoryRuleExcludeList{"templates/vendor/"}, excludes.Directories)
}

// TestRemapHTTPSCertificateReuseRuleLevel proves the rule-level impact
// override (global org-wide config, mirroring how every other Templates
// rule's level is wired) reaches pkg.TemplatesLinterRules too.
func TestRemapHTTPSCertificateReuseRuleLevel(t *testing.T) {
	settings := remapLinterSettings(&config.LintersSettings{}, &global.Linters{
		Templates: global.TemplatesLinterConfig{
			Rules: global.TemplatesLinterRules{
				HTTPSCertificateReuseRule: global.RuleConfig{Impact: pkg.Warn.String()},
			},
		},
	})

	require.Equal(t, pkg.Warn, *settings.Templates.Rules.HTTPSCertificateReuseRule.GetLevel())
}
