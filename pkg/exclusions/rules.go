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

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
)

// TrackedStringRule extends StringRule with exclusion tracking
type TrackedStringRule struct {
	pkg.StringRule
	tracker  *ExclusionTracker
	linterID string
	ruleID   string
}

// NewTrackedStringRule creates a new tracked string rule
func NewTrackedStringRule(excludeRules []pkg.StringRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *TrackedStringRule {
	return NewTrackedStringRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

// NewTrackedStringRuleForModule creates a new tracked string rule for a specific module
func NewTrackedStringRuleForModule(excludeRules []pkg.StringRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *TrackedStringRule {
	// Register all exclusions with the tracker
	exclusions := make([]string, len(excludeRules))
	for i, rule := range excludeRules {
		exclusions[i] = string(rule)
	}
	tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)

	return &TrackedStringRule{
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

// Enabled checks if the rule is enabled for the given string and tracks usage
func (r *TrackedStringRule) Enabled(str string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(str) { // If rule.Enabled returns false, exclusion matched
			// Mark this exclusion as used
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, string(rule))
			return false
		}
	}
	return true
}

// TrackedPrefixRule extends PrefixRule with exclusion tracking
type TrackedPrefixRule struct {
	pkg.PrefixRule
	tracker  *ExclusionTracker
	linterID string
	ruleID   string
}

// NewTrackedPrefixRule creates a new tracked prefix rule
func NewTrackedPrefixRule(excludeRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *TrackedPrefixRule {
	return NewTrackedPrefixRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

// NewTrackedPrefixRuleForModule creates a new tracked prefix rule for a specific module
func NewTrackedPrefixRuleForModule(excludeRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *TrackedPrefixRule {
	// Register all exclusions with the tracker
	exclusions := make([]string, len(excludeRules))
	for i, rule := range excludeRules {
		exclusions[i] = string(rule)
	}
	tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)

	return &TrackedPrefixRule{
		PrefixRule: pkg.PrefixRule{
			ExcludeRules: excludeRules,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

// Enabled checks if the rule is enabled for the given string and tracks usage
func (r *TrackedPrefixRule) Enabled(str string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(str) { // If rule.Enabled returns false, exclusion matched
			// Mark this exclusion as used
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, string(rule))
			return false
		}
	}
	return true
}

// TrackedKindRule extends KindRule with exclusion tracking
type TrackedKindRule struct {
	pkg.KindRule
	tracker  *ExclusionTracker
	linterID string
	ruleID   string
}

// NewTrackedKindRule creates a new tracked kind rule
func NewTrackedKindRule(excludeRules []pkg.KindRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *TrackedKindRule {
	return NewTrackedKindRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

// NewTrackedKindRuleForModule creates a new tracked kind rule for a specific module
func NewTrackedKindRuleForModule(excludeRules []pkg.KindRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *TrackedKindRule {
	// Register all exclusions with the tracker
	exclusions := make([]string, len(excludeRules))
	for i, rule := range excludeRules {
		exclusions[i] = fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
	}
	tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)

	return &TrackedKindRule{
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

// Enabled checks if the rule is enabled for the given kind and name and tracks usage
func (r *TrackedKindRule) Enabled(kind, name string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(kind, name) { // If rule.Enabled returns false, exclusion matched
			// Mark this exclusion as used
			exclusionKey := fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, exclusionKey)
			return false
		}
	}
	return true
}

// TrackedContainerRule extends ContainerRule with exclusion tracking
type TrackedContainerRule struct {
	pkg.ContainerRule
	tracker  *ExclusionTracker
	linterID string
	ruleID   string
}

// NewTrackedContainerRule creates a new tracked container rule
func NewTrackedContainerRule(excludeRules []pkg.ContainerRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *TrackedContainerRule {
	return NewTrackedContainerRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

// NewTrackedContainerRuleForModule creates a new tracked container rule for a specific module
func NewTrackedContainerRuleForModule(excludeRules []pkg.ContainerRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *TrackedContainerRule {
	// Register all exclusions with the tracker
	exclusions := make([]string, len(excludeRules))
	for i, rule := range excludeRules {
		if rule.Container == "" {
			exclusions[i] = fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
		} else {
			exclusions[i] = fmt.Sprintf("%s/%s/%s", rule.Kind, rule.Name, rule.Container)
		}
	}
	tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)

	return &TrackedContainerRule{
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

// Enabled checks if the rule is enabled for the given object and container and tracks usage
func (r *TrackedContainerRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(object, container) { // If rule.Enabled returns false, exclusion matched
			// Mark this exclusion as used
			var exclusionKey string
			if rule.Container == "" {
				exclusionKey = fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
			} else {
				exclusionKey = fmt.Sprintf("%s/%s/%s", rule.Kind, rule.Name, rule.Container)
			}
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, exclusionKey)
			return false
		}
	}
	return true
}

// TrackedServicePortRule extends ServicePortRule with exclusion tracking
type TrackedServicePortRule struct {
	pkg.ServicePortRule
	tracker  *ExclusionTracker
	linterID string
	ruleID   string
}

// NewTrackedServicePortRule creates a new tracked service port rule
func NewTrackedServicePortRule(excludeRules []pkg.ServicePortExclude, tracker *ExclusionTracker, linterID, ruleID string) *TrackedServicePortRule {
	return NewTrackedServicePortRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

// NewTrackedServicePortRuleForModule creates a new tracked service port rule for a specific module
func NewTrackedServicePortRuleForModule(excludeRules []pkg.ServicePortExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *TrackedServicePortRule {
	// Register all exclusions with the tracker
	exclusions := make([]string, len(excludeRules))
	for i, rule := range excludeRules {
		exclusions[i] = fmt.Sprintf("%s:%s", rule.Name, rule.Port)
	}
	tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)

	return &TrackedServicePortRule{
		ServicePortRule: pkg.ServicePortRule{
			ExcludeRules: excludeRules,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

// Enabled checks if the rule is enabled for the given name and port and tracks usage
func (r *TrackedServicePortRule) Enabled(name, port string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(name, port) { // If rule.Enabled returns false, exclusion matched
			// Mark this exclusion as used
			exclusionKey := fmt.Sprintf("%s:%s", rule.Name, rule.Port)
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, exclusionKey)
			return false
		}
	}
	return true
}

// TrackedPathRule extends PathRule with exclusion tracking
type TrackedPathRule struct {
	pkg.PathRule
	tracker  *ExclusionTracker
	linterID string
	ruleID   string
}

// NewTrackedPathRule creates a new tracked path rule
func NewTrackedPathRule(excludeStringRules []pkg.StringRuleExclude, excludePrefixRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *TrackedPathRule {
	return NewTrackedPathRuleForModule(excludeStringRules, excludePrefixRules, tracker, linterID, ruleID, "")
}

// NewTrackedPathRuleForModule creates a new tracked path rule for a specific module
func NewTrackedPathRuleForModule(excludeStringRules []pkg.StringRuleExclude, excludePrefixRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *TrackedPathRule {
	// Register all exclusions with the tracker
	exclusions := make([]string, 0, len(excludeStringRules)+len(excludePrefixRules))
	for _, rule := range excludeStringRules {
		exclusions = append(exclusions, string(rule))
	}
	for _, rule := range excludePrefixRules {
		exclusions = append(exclusions, string(rule))
	}
	tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)

	return &TrackedPathRule{
		PathRule: pkg.PathRule{
			ExcludeStringRules: excludeStringRules,
			ExcludePrefixRules: excludePrefixRules,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

// Enabled checks if the rule is enabled for the given name and tracks usage
func (r *TrackedPathRule) Enabled(name string) bool {
	for _, rule := range r.ExcludeStringRules {
		if !rule.Enabled(name) { // If rule.Enabled returns false, exclusion matched
			// Mark this exclusion as used
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, string(rule))
			return false
		}
	}

	for _, rule := range r.ExcludePrefixRules {
		if !rule.Enabled(name) { // If rule.Enabled returns false, exclusion matched
			// Mark this exclusion as used
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, string(rule))
			return false
		}
	}

	return true
}

// TrackedBoolRule extends BoolRule with exclusion tracking
type TrackedBoolRule struct {
	pkg.BoolRule
	tracker  *ExclusionTracker
	linterID string
	ruleID   string
}

// NewTrackedBoolRule creates a new tracked bool rule
func NewTrackedBoolRule(disable bool, tracker *ExclusionTracker, linterID, ruleID string) *TrackedBoolRule {
	return NewTrackedBoolRuleForModule(disable, tracker, linterID, ruleID, "")
}

// NewTrackedBoolRuleForModule creates a new tracked bool rule for a specific module
func NewTrackedBoolRuleForModule(disable bool, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *TrackedBoolRule {
	// Register the rule in tracker (bool rules don't have exclusions, but we track them)
	tracker.RegisterExclusionsForModule(linterID, ruleID, []string{}, moduleName)

	return &TrackedBoolRule{
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

// Enabled checks if the rule is enabled and tracks usage
func (r *TrackedBoolRule) Enabled() bool {
	if r.Exclude {
		// Mark this exclusion as used
		r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, "disabled")
		return false
	}
	return true
}
