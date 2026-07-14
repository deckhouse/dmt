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
	"strings"
)

// combinations returns index-tuples (one value index per axis) to render. When
// the full cartesian product fits within limit it is returned in full;
// otherwise an all-pairs set is produced so every pair of axis values still
// co-occurs in at least one tuple, then capped at limit.
func combinations(axes []Axis, limit int) [][]int {
	if len(axes) == 0 || limit <= 0 {
		return nil
	}

	if size, ok := cartesianSize(axes, limit); ok {
		return cartesian(axes, size)
	}

	combos := pairwise(axes)
	if len(combos) > limit {
		combos = combos[:limit]
	}

	return combos
}

// cartesianSize returns the product of axis lengths, and false if it exceeds
// limit (short-circuiting to avoid overflow on pathological schemas).
func cartesianSize(axes []Axis, limit int) (int, bool) {
	size := 1

	for i := range axes {
		size *= len(axes[i].Values)
		if size > limit {
			return 0, false
		}
	}

	return size, true
}

func cartesian(axes []Axis, size int) [][]int {
	combos := make([][]int, 0, size)
	idx := make([]int, len(axes))

	for {
		combo := make([]int, len(axes))
		copy(combo, idx)
		combos = append(combos, combo)

		// increment the mixed-radix counter
		pos := len(axes) - 1
		for pos >= 0 {
			idx[pos]++
			if idx[pos] < len(axes[pos].Values) {
				break
			}

			idx[pos] = 0
			pos--
		}

		if pos < 0 {
			break
		}
	}

	return combos
}

// pairwise returns a set of tuples covering every pair of (axis, value) across
// all axis pairs. It is a simple, deterministic all-pairs construction: for each
// pair of axes it emits a tuple pinning those two axes to each value
// combination while leaving the rest at their first value. Duplicate tuples are
// removed. This guarantees 2-way coverage, which is enough to reach resources
// gated by two simultaneous conditions.
func pairwise(axes []Axis) [][]int {
	seen := map[string]struct{}{}
	var combos [][]int

	add := func(combo []int) {
		key := comboKey(combo)
		if _, dup := seen[key]; dup {
			return
		}

		seen[key] = struct{}{}
		combos = append(combos, combo)
	}

	base := make([]int, len(axes)) // all firsts

	// Single-axis sweeps first: every value of every axis appears at least once.
	for i := range axes {
		for v := 1; v < len(axes[i].Values); v++ {
			combo := append([]int(nil), base...)
			combo[i] = v
			add(combo)
		}
	}

	// Then every pair of axes at every value combination.
	for i := range axes {
		for j := i + 1; j < len(axes); j++ {
			for vi := range axes[i].Values {
				for vj := range axes[j].Values {
					combo := append([]int(nil), base...)
					combo[i] = vi
					combo[j] = vj
					add(combo)
				}
			}
		}
	}

	if len(combos) == 0 {
		add(append([]int(nil), base...))
	}

	return combos
}

func comboKey(combo []int) string {
	parts := make([]string, len(combo))
	for i, v := range combo {
		parts[i] = fmt.Sprintf("%d", v)
	}

	return strings.Join(parts, ",")
}
