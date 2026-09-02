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

package scopes

import (
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/docs"
	docsrules "github.com/deckhouse/dmt/pkg/linters/docs/rules"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	modulerules "github.com/deckhouse/dmt/pkg/linters/module/rules"
)

// bundleRules is the rule membership of the bundle scope: the bundle image
// (<repo>:<tag>) holds the packaged module — chart, templates, docs and digests —
// so bundle-layout asks for the whole of that shape. The changelog rule is not part
// of it: changelog.yaml ships in the release image, and release-layout is what makes
// its absence a finding there.
//
// What it deliberately does not ask for is anything under the templates or container
// linters. A bundle carries rendered-looking directories but the module behind this
// scope comes from modules.NewRemoteModule, whose object store is nil; those linters
// would work off a chart that was never loaded.
var bundleRules = map[string]set.Set{
	moduleLinter.ID: set.New(
		modulerules.BundleLayoutRuleName,
	),
	docs.ID: set.New(
		docsrules.ReadmeRuleName,
	),
}

// bundleLinters builds the linters of the bundle scope, handing each one its slice of
// the module config and the rule IDs bundleRules asks it for.
func bundleLinters(m *modules.Module, errList *errors.LintRuleErrorsList) []Linter {
	cfg := m.GetModuleConfig()
	if cfg == nil {
		cfg = &pkg.LintersSettings{}
	}

	return []Linter{
		moduleLinter.New(&cfg.Module, bundleRules[moduleLinter.ID], m, errList),
		docs.New(&cfg.Documentation, bundleRules[docs.ID], m, errList),
	}
}
