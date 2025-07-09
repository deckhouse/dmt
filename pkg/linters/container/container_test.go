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

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestContainer_NameAndDesc(t *testing.T) {
	cfg := &config.ModuleConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, errList, nil)

	assert.Equal(t, ID, linter.Name(), "Name() should return linter ID")
	assert.Equal(t, "Lint container objects", linter.Desc(), "Desc() should return linter description")
}

func TestContainer_Run_NilModule(_ *testing.T) {
	cfg := &config.ModuleConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, errList, nil)

	// Should not panic or fail if module is nil
	linter.Run(nil)
}

func TestContainer_Run_EmptyModule(t *testing.T) {
	cfg := &config.ModuleConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := New(cfg, errList, nil)

	mod := &module.Module{} // Module with nil objectStore
	linter.Run(mod)
	// No errors expected
	assert.Empty(t, errList.GetErrors())
}
