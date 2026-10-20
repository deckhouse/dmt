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

// TestRemapIngressExcludeRules proves that .dmtlint.yaml ingress kind/name and
// ingress-paths excludes parsed into config.TemplatesExcludeRules reach the
// lists the ingress rule reads.
func TestRemapIngressExcludeRules(t *testing.T) {
	configSettings := &config.LintersSettings{
		Templates: config.TemplatesSettings{
			ExcludeRules: config.TemplatesExcludeRules{
				Ingress: config.KindRuleExcludeList{
					{Kind: "Ingress", Name: "legacy"},
				},
				IngressPaths: config.PathRuleExclude{
					Files:       config.StringRuleExcludeList{"templates/legacy-ingress.yaml"},
					Directories: config.DirectoryRuleExcludeList{"templates/legacy"},
				},
			},
		},
	}

	settings := remapLinterSettings(configSettings, &global.Linters{})

	require.Equal(t, pkg.KindRuleExcludeList{{Kind: "Ingress", Name: "legacy"}}, settings.Templates.ExcludeRules.Ingress)
	require.Equal(t, pkg.StringRuleExcludeList{"templates/legacy-ingress.yaml"}, settings.Templates.ExcludeRules.IngressPaths.Files)
	require.Equal(t, pkg.DirectoryRuleExcludeList{"templates/legacy"}, settings.Templates.ExcludeRules.IngressPaths.Directories)
}
