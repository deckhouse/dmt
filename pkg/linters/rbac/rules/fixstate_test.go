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

package rules

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/bootstrap"
)

// resetFixState forgets everything; tests call it between runs. The two locks are taken one after
// the other, never one inside the other: fixOnce holds fixOutcomes while its fix takes fixState.
func resetFixState() {
	fixState.Lock()
	fixState.withheld = set.New()
	fixState.changes = map[string]set.Set{}
	fixState.bootstrap = map[string]map[string]bootstrap.Object{}
	fixState.variants = map[string]int{}
	fixState.seen = map[string]map[string]int{}
	fixState.in = map[string]map[string]string{}
	fixState.Unlock()

	fixOutcomes.Lock()
	fixOutcomes.done = map[string]error{}
	fixOutcomes.Unlock()
}

// fixOnce runs the fix of a target once, however many render variants ask, and returns its outcome
// to every caller.
func TestFixOnce(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	calls := 0
	failing := stderrors.New("write failed")

	for range 3 {
		err := fixOnce("target", func() error {
			calls++

			return failing
		})
		require.ErrorIs(t, err, failing)
	}

	assert.Equal(t, 1, calls)
}
