package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestContainer_NameAndDesc(t *testing.T) {
	cfg := &pkg.ContainerLinterConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, nil, nil, errList)

	assert.Equal(t, ID, linter.GetName(), "GetName() should return linter ID")
	assert.Equal(t, "Lint container objects", linter.Desc(), "Desc() should return linter description")
}

func TestContainer_Lint_EmptyModule(t *testing.T) {
	cfg := &pkg.ContainerLinterConfig{}
	errList := errors.NewLintRuleErrorsList()

	mod := &modules.Module{} // Module with nil objectStore
	linter := New(cfg, everyRule(cfg, mod), mod, errList)
	linter.Lint(t.Context())
	// No errors expected
	assert.Empty(t, errList.GetErrors())
}

// everyRule returns the IDs of every rule the linter builds for cfg and m.
//
// It is test-only and derived from rules() rather than written out, so it cannot go stale.
// A scope picks the rules it wants; these tests want the linter exercised whole, and
// naming a subset here would let a rule silently drop out of what they cover.
func everyRule(cfg *pkg.ContainerLinterConfig, m pkg.Module) set.Set {
	ids := set.New()

	probe := New(cfg, nil, m, errors.NewLintRuleErrorsList())
	for _, rule := range probe.rules(nil) {
		ids.Add(rule.GetName())
	}

	return ids
}
