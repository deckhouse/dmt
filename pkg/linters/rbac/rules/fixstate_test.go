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
