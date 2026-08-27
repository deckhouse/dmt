package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestContainer_NameAndDesc(t *testing.T) {
	cfg := &pkg.ContainerLinterConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, AllRuleNames(), nil, errList)

	assert.Equal(t, ID, linter.GetName(), "GetName() should return linter ID")
	assert.Equal(t, "Lint container objects", linter.Desc(), "Desc() should return linter description")
}

func TestContainer_Lint_EmptyModule(t *testing.T) {
	cfg := &pkg.ContainerLinterConfig{}
	errList := errors.NewLintRuleErrorsList()

	mod := &modules.Module{} // Module with nil objectStore
	linter := New(cfg, AllRuleNames(), mod, errList)
	linter.Lint(t.Context())
	// No errors expected
	assert.Empty(t, errList.GetErrors())
}
