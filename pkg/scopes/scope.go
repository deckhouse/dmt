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

// Package scopes defines the verification scopes dmt lints in.
//
// A scope is a source a module is read from, and it decides both which linters run and
// which of their rules each one is asked for. Linters and rules know nothing about
// scopes: a linter is handed its config and a set of rule IDs, and that is the whole of
// what a scope tells it. Which rules run is therefore a property of the tool, not of a
// module — .dmtlint.yaml tunes severities and can silence a rule with an ignored impact,
// but it cannot switch one on or off.
//
// This mirrors internal/verify/lint in d8-package-plugin with the declaration inverted:
// there every linter exports its own scope table and gates its rules itself, so each
// linter has to know the scopes it lives in. Here the scope owns both tables, which is
// what keeps the linters unaware.
package scopes

import (
	"context"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Scope names one verification target: the source a module is read from.
type Scope string

// Static lints a module directory on disk, i.e. the full source tree as committed.
// Release and Bundle lint the two images a published module consists of, unpacked
// from the registry: the bundle is the packaged module itself, the release is the
// metadata Deckhouse reads to decide whether to install a version.
const (
	Static  Scope = "static"
	Release Scope = "release"
	Bundle  Scope = "bundle"
)

// Settings returns the linter settings that configure this scope. Each scope reads its
// own section of .dmtlint.yaml — `linters-settings` under `global` for the source tree,
// `remote.bundle` and `remote.release` for the two published images — so the same rule
// can carry a different severity depending on where it is checked. The sections are
// independent: an image is not linted with the severities the source tree was tuned to,
// and a section left out means the built-in defaults.
func (s Scope) Settings(cfg *config.RootConfig) *global.Linters {
	switch s {
	case Release:
		return &cfg.Remote.Release
	case Bundle:
		return &cfg.Remote.Bundle
	default:
		// The loader always fills GlobalSettings in, but a RootConfig built by hand can
		// leave it nil. An empty tree is the right answer there: every impact remaps to
		// its default, which is what a missing config means everywhere else.
		if cfg.GlobalSettings == nil {
			return &global.Linters{}
		}

		return &cfg.GlobalSettings.Linters
	}
}

// Linter is the common interface implemented by all lint passes. Everything a linter
// needs — its config, the rule IDs the scope asked it for, the module it inspects and
// the error list it reports into — is supplied to its constructor, so Lint takes only a
// context.
type Linter interface {
	GetName() string
	Lint(ctx context.Context)
}

// Linters returns the linters this scope runs over the module, each already carrying the
// set of rule IDs the scope asked it for.
//
// It takes *modules.Module rather than pkg.Module because GetModuleConfig lives on the
// concrete type only. That is deliberate: slicing the config per linter is the scope's
// job, and a linter must not be able to reach a sibling's settings.
func (s Scope) Linters(m *modules.Module, errList *errors.LintRuleErrorsList) []Linter {
	switch s {
	case Release:
		return releaseLinters(m, errList)
	case Bundle:
		return bundleLinters(m, errList)
	default:
		return staticLinters(m, errList)
	}
}
