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

package matrix

import (
	"fmt"
	"testing"
)

func axesOfSize(sizes ...int) []Axis {
	axes := make([]Axis, len(sizes))

	for i, n := range sizes {
		vals := make([]any, n)
		for v := range vals {
			vals[v] = v
		}

		axes[i] = Axis{Path: []string{fmt.Sprintf("a%d", i)}, Values: vals}
	}

	return axes
}

func TestCombinations_FullCartesian(t *testing.T) {
	axes := axesOfSize(2, 3, 2) // product = 12

	combos := combinations(axes, 100)
	if len(combos) != 12 {
		t.Fatalf("expected 12 cartesian combos, got %d", len(combos))
	}

	// All must be unique.
	seen := map[string]struct{}{}
	for _, c := range combos {
		k := comboKey(c)
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate combo %v", c)
		}

		seen[k] = struct{}{}
	}
}

func TestCombinations_PairwiseFallbackCoversAllPairs(t *testing.T) {
	// 5 booleans => cartesian 32; a limit below that forces the pairwise
	// fallback, while still leaving room for the full all-pairs set (~16).
	axes := axesOfSize(2, 2, 2, 2, 2)

	const limit = 25

	combos := combinations(axes, limit)

	if len(combos) >= 32 {
		t.Fatalf("expected pairwise fallback (fewer than cartesian 32), got %d", len(combos))
	}

	if len(combos) > limit {
		t.Fatalf("pairwise result exceeded limit: %d", len(combos))
	}

	// Every pair (i,j) and every (vi,vj) must appear in some combo.
	for i := range axes {
		for j := i + 1; j < len(axes); j++ {
			for vi := range axes[i].Values {
				for vj := range axes[j].Values {
					if !pairCovered(combos, i, j, vi, vj) {
						t.Fatalf("pair (a%d=%d, a%d=%d) not covered", i, vi, j, vj)
					}
				}
			}
		}
	}
}

func pairCovered(combos [][]int, i, j, vi, vj int) bool {
	for _, c := range combos {
		if c[i] == vi && c[j] == vj {
			return true
		}
	}

	return false
}
