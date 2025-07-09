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

package exclusions

import (
	"fmt"
	"strings"
	"sync"

	"github.com/deckhouse/dmt/pkg"
)

// Ensure ExclusionTracker implements pkg.ExclusionTracker interface
var _ pkg.ExclusionTracker = (*ExclusionTracker)(nil)

// ExclusionTracker tracks which exclusions are used during linting
type ExclusionTracker struct {
	mu sync.RWMutex
	// Map of linter -> rule -> exclusion -> usage count
	usedExclusions map[string]map[string]map[string]int
	// Map of linter -> rule -> all configured exclusions
	configuredExclusions map[string]map[string][]string
	// Map of linter -> rule -> exclusion -> module names
	exclusionModules map[string]map[string]map[string][]string
}

// NewExclusionTracker creates a new exclusion tracker
func NewExclusionTracker() *ExclusionTracker {
	return &ExclusionTracker{
		usedExclusions:       make(map[string]map[string]map[string]int),
		configuredExclusions: make(map[string]map[string][]string),
		exclusionModules:     make(map[string]map[string]map[string][]string),
	}
}

// RegisterExclusions registers all exclusions for a specific linter and rule
func (t *ExclusionTracker) RegisterExclusions(linterID, ruleID string, exclusions []string) {
	t.RegisterExclusionsForModule(linterID, ruleID, exclusions, "")
}

// RegisterExclusionsForModule registers all exclusions for a specific linter, rule and module
func (t *ExclusionTracker) RegisterExclusionsForModule(linterID, ruleID string, exclusions []string, moduleName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.configuredExclusions[linterID] == nil {
		t.configuredExclusions[linterID] = make(map[string][]string)
	}
	if t.exclusionModules[linterID] == nil {
		t.exclusionModules[linterID] = make(map[string]map[string][]string)
	}

	// Append exclusions instead of overwriting them
	if t.configuredExclusions[linterID][ruleID] == nil {
		t.configuredExclusions[linterID][ruleID] = make([]string, 0)
	}
	if t.exclusionModules[linterID][ruleID] == nil {
		t.exclusionModules[linterID][ruleID] = make(map[string][]string)
	}

	// Add new exclusions to existing ones, avoiding duplicates
	existing := make(map[string]bool)
	for _, excl := range t.configuredExclusions[linterID][ruleID] {
		existing[excl] = true
	}

	for _, excl := range exclusions {
		if !existing[excl] {
			t.configuredExclusions[linterID][ruleID] = append(t.configuredExclusions[linterID][ruleID], excl)
		}
		// Always track module association
		if moduleName != "" {
			if t.exclusionModules[linterID][ruleID][excl] == nil {
				t.exclusionModules[linterID][ruleID][excl] = make([]string, 0)
			}
			// Check if module is already in the list
			found := false
			for _, existingModule := range t.exclusionModules[linterID][ruleID][excl] {
				if existingModule == moduleName {
					found = true
					break
				}
			}
			if !found {
				t.exclusionModules[linterID][ruleID][excl] = append(t.exclusionModules[linterID][ruleID][excl], moduleName)
			}
		}
	}
}

// MarkExclusionUsed marks an exclusion as used for a specific linter and rule
func (t *ExclusionTracker) MarkExclusionUsed(linterID, ruleID, exclusion string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.usedExclusions[linterID] == nil {
		t.usedExclusions[linterID] = make(map[string]map[string]int)
	}
	if t.usedExclusions[linterID][ruleID] == nil {
		t.usedExclusions[linterID][ruleID] = make(map[string]int)
	}
	t.usedExclusions[linterID][ruleID][exclusion]++
}

// GetUnusedExclusions returns all exclusions that were configured but never used
func (t *ExclusionTracker) GetUnusedExclusions() map[string]map[string][]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	unused := make(map[string]map[string][]string)

	for linterID, rules := range t.configuredExclusions {
		for ruleID, exclusions := range rules {
			unusedForRule := make([]string, 0)

			for _, exclusion := range exclusions {
				if t.usedExclusions[linterID] == nil ||
					t.usedExclusions[linterID][ruleID] == nil ||
					t.usedExclusions[linterID][ruleID][exclusion] == 0 {
					unusedForRule = append(unusedForRule, exclusion)
				}
			}

			if len(unusedForRule) > 0 {
				if unused[linterID] == nil {
					unused[linterID] = make(map[string][]string)
				}
				unused[linterID][ruleID] = unusedForRule
			}
		}
	}

	return unused
}

// GetUsageStats returns usage statistics for all exclusions
func (t *ExclusionTracker) GetUsageStats() map[string]map[string]map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	result := make(map[string]map[string]map[string]int)

	for linterID, rules := range t.usedExclusions {
		result[linterID] = make(map[string]map[string]int)
		for ruleID, exclusions := range rules {
			result[linterID][ruleID] = make(map[string]int)
			for exclusion, count := range exclusions {
				result[linterID][ruleID][exclusion] = count
			}
		}
	}

	return result
}

// FormatUnusedExclusions formats unused exclusions for warning output
func (t *ExclusionTracker) FormatUnusedExclusions() string {
	unused := t.GetUnusedExclusions()

	if len(unused) == 0 {
		return ""
	}

	result := "Unused exclusions found:\n"

	for linterID, rules := range unused {
		result += fmt.Sprintf("  %s:\n", linterID)
		for ruleID, exclusions := range rules {
			result += fmt.Sprintf("    %s:\n", ruleID)
			for _, exclusion := range exclusions {
				moduleInfo := ""
				if t.exclusionModules[linterID] != nil &&
					t.exclusionModules[linterID][ruleID] != nil &&
					t.exclusionModules[linterID][ruleID][exclusion] != nil {
					modules := t.exclusionModules[linterID][ruleID][exclusion]
					if len(modules) > 0 {
						moduleInfo = fmt.Sprintf(" (from modules: %s)", strings.Join(modules, ", "))
					}
				}
				result += fmt.Sprintf("      - %s%s\n", exclusion, moduleInfo)
			}
		}
	}

	return result
}
