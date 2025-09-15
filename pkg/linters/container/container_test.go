package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestContainer_NameAndDesc(t *testing.T) {
	errorLevel := pkg.Error
	cfg := &config.ModuleConfig{
		LintersSettings: &config.LintersSettings{
			Container: config.ContainerSettings{
				Impact: &errorLevel,
			},
		},
	}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, errList)

	assert.Equal(t, ID, linter.Name(), "Name() should return linter ID")
	assert.Equal(t, "Lint container objects", linter.Desc(), "Desc() should return linter description")
}

func TestContainer_Run_NilModule(_ *testing.T) {
	errorLevel := pkg.Error
	cfg := &config.ModuleConfig{
		LintersSettings: &config.LintersSettings{
			Container: config.ContainerSettings{
				Impact: &errorLevel,
			},
		},
	}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, errList)

	// Should not panic or fail if module is nil
	linter.Run(nil)
}

func TestContainer_Run_EmptyModule(t *testing.T) {
	errorLevel := pkg.Error
	cfg := &config.ModuleConfig{
		LintersSettings: &config.LintersSettings{
			Container: config.ContainerSettings{
				Impact: &errorLevel,
			},
		},
	}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, errList)

	mod := &module.Module{} // Module with nil objectStore
	linter.Run(mod)
	// No errors expected
	assert.Empty(t, errList.GetErrors())
}
