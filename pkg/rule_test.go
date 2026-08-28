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

package pkg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/set"
)

// spyRule records whether RunRules checked it.
type spyRule struct {
	RuleMeta

	checked bool
}

func (r *spyRule) Check(context.Context) { r.checked = true }

var _ Rule = (*spyRule)(nil)

func TestRunRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		want set.Set
		run  []string
	}{
		{
			name: "checks only what was asked for",
			want: set.New("a", "c"),
			run:  []string{"a", "c"},
		},
		{
			// The invariant an explicit allowlist rests on: no request, no rule. Were an
			// empty set to mean "everything", a missing table entry would silently run
			// every rule instead of failing loudly.
			name: "an empty set checks nothing",
			want: set.New(),
			run:  nil,
		},
		{
			name: "a nil set checks nothing",
			want: nil,
			run:  nil,
		},
		{
			name: "an ID no rule carries is ignored",
			want: set.New("a", "typo"),
			run:  []string{"a"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules := []Rule{
				&spyRule{RuleMeta: RuleMeta{Name: "a"}},
				&spyRule{RuleMeta: RuleMeta{Name: "b"}},
				&spyRule{RuleMeta: RuleMeta{Name: "c"}},
			}

			RunRules(t.Context(), tc.want, rules)

			var checked []string

			for _, r := range rules {
				if spy := r.(*spyRule); spy.checked {
					checked = append(checked, spy.Name)
				}
			}

			assert.Equal(t, tc.run, checked)
		})
	}
}
