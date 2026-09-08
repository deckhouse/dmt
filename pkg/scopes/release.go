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
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	modulerules "github.com/deckhouse/dmt/pkg/linters/module/rules"
)

// releaseRules is the rule membership of the release scope: the release image
// (<repo>/release:<tag>) holds only the metadata Deckhouse reads to decide whether
// to install a version, so the scope asks for the rules that live in that metadata
// and for nothing that needs a chart or a rendered object — the module behind this
// scope is built by modules.NewRemoteModule and has neither.
//
// No rule here turns a missing file into an error. definition-file, package-yaml and
// changelog-valid validate the contents of module.yaml, package.yaml and changelog.yaml
// and stay quiet when the file is absent; has-changelog is the only one that reports an
// absence at all, and it warns. So this scope checks what the image ships, not what it
// forgot to ship — an empty release image comes back with a single changelog warning.
var releaseRules = map[string]set.Set{
	moduleLinter.ID: set.New(
		modulerules.DefinitionFileRuleName,
		modulerules.PackageYAMLRuleName,
		modulerules.HasChangelogRuleName,
		modulerules.ChangelogValidRuleName,
	),
}

// releaseLinters builds the linters of the release scope, handing each one its slice
// of the module config and the rule IDs releaseRules asks it for.
func releaseLinters(m *modules.Module, errList *errors.LintRuleErrorsList) []Linter {
	cfg := m.GetModuleConfig()
	if cfg == nil {
		cfg = &pkg.LintersSettings{}
	}

	return []Linter{
		moduleLinter.New(&cfg.Module, releaseRules[moduleLinter.ID], m, errList),
	}
}
