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

	"github.com/deckhouse/dmt/pkg"
)

// Universal tracked rule constructor
func NewTrackedRule[T any](rule *T, exclusions []string, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *T {
	if tracker != nil {
		tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)
	}
	return rule
}

// --- Key generators ---
func StringRuleKeys(rules []pkg.StringRuleExclude) []string {
	res := make([]string, len(rules))
	for i, rule := range rules {
		res[i] = string(rule)
	}
	return res
}

func PrefixRuleKeys(rules []pkg.PrefixRuleExclude) []string {
	res := make([]string, len(rules))
	for i, rule := range rules {
		res[i] = string(rule)
	}
	return res
}

func KindRuleKeys(rules []pkg.KindRuleExclude) []string {
	res := make([]string, len(rules))
	for i, rule := range rules {
		res[i] = fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
	}
	return res
}

func ContainerRuleKeys(rules []pkg.ContainerRuleExclude) []string {
	res := make([]string, len(rules))
	for i, rule := range rules {
		if rule.Container == "" {
			res[i] = fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
		} else {
			res[i] = fmt.Sprintf("%s/%s/%s", rule.Kind, rule.Name, rule.Container)
		}
	}
	return res
}

func ServicePortRuleKeys(rules []pkg.ServicePortExclude) []string {
	res := make([]string, len(rules))
	for i, rule := range rules {
		res[i] = fmt.Sprintf("%s:%s", rule.Name, rule.Port)
	}
	return res
}

func PathRuleKeys(stringRules []pkg.StringRuleExclude, prefixRules []pkg.PrefixRuleExclude) []string {
	res := make([]string, 0, len(stringRules)+len(prefixRules))
	for _, rule := range stringRules {
		res = append(res, string(rule))
	}
	for _, rule := range prefixRules {
		res = append(res, string(rule))
	}
	return res
}

// --- Aliases for backward compatibility ---

func NewTrackedStringRuleForModule(excludeRules []pkg.StringRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *pkg.StringRule {
	return NewTrackedRule(pkg.NewStringRuleWithTracker(excludeRules, tracker, linterID, ruleID), StringRuleKeys(excludeRules), tracker, linterID, ruleID, moduleName)
}

func NewTrackedStringRule(excludeRules []pkg.StringRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *pkg.StringRule {
	return NewTrackedStringRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

func NewTrackedPrefixRuleForModule(excludeRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *pkg.PrefixRule {
	return NewTrackedRule(pkg.NewPrefixRuleWithTracker(excludeRules, tracker, linterID, ruleID), PrefixRuleKeys(excludeRules), tracker, linterID, ruleID, moduleName)
}

func NewTrackedPrefixRule(excludeRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *pkg.PrefixRule {
	return NewTrackedPrefixRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

func NewTrackedKindRuleForModule(excludeRules []pkg.KindRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *pkg.KindRule {
	return NewTrackedRule(pkg.NewKindRuleWithTracker(excludeRules, tracker, linterID, ruleID), KindRuleKeys(excludeRules), tracker, linterID, ruleID, moduleName)
}

func NewTrackedKindRule(excludeRules []pkg.KindRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *pkg.KindRule {
	return NewTrackedKindRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

func NewTrackedContainerRuleForModule(excludeRules []pkg.ContainerRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *pkg.ContainerRule {
	return NewTrackedRule(pkg.NewContainerRuleWithTracker(excludeRules, tracker, linterID, ruleID), ContainerRuleKeys(excludeRules), tracker, linterID, ruleID, moduleName)
}

func NewTrackedContainerRule(excludeRules []pkg.ContainerRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *pkg.ContainerRule {
	return NewTrackedContainerRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

func NewTrackedServicePortRuleForModule(excludeRules []pkg.ServicePortExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *pkg.ServicePortRule {
	return NewTrackedRule(pkg.NewServicePortRuleWithTracker(excludeRules, tracker, linterID, ruleID), ServicePortRuleKeys(excludeRules), tracker, linterID, ruleID, moduleName)
}

func NewTrackedServicePortRule(excludeRules []pkg.ServicePortExclude, tracker *ExclusionTracker, linterID, ruleID string) *pkg.ServicePortRule {
	return NewTrackedServicePortRuleForModule(excludeRules, tracker, linterID, ruleID, "")
}

func NewTrackedPathRuleForModule(excludeStringRules []pkg.StringRuleExclude, excludePrefixRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *pkg.PathRule {
	return NewTrackedRule(pkg.NewPathRuleWithTracker(excludeStringRules, excludePrefixRules, tracker, linterID, ruleID), PathRuleKeys(excludeStringRules, excludePrefixRules), tracker, linterID, ruleID, moduleName)
}

func NewTrackedPathRule(excludeStringRules []pkg.StringRuleExclude, excludePrefixRules []pkg.PrefixRuleExclude, tracker *ExclusionTracker, linterID, ruleID string) *pkg.PathRule {
	return NewTrackedPathRuleForModule(excludeStringRules, excludePrefixRules, tracker, linterID, ruleID, "")
}

func NewTrackedBoolRuleForModule(disable bool, tracker *ExclusionTracker, linterID, ruleID, moduleName string) *pkg.BoolRule {
	return NewTrackedRule(pkg.NewBoolRuleWithTracker(disable, tracker, linterID, ruleID), []string{}, tracker, linterID, ruleID, moduleName)
}

func NewTrackedBoolRule(disable bool, tracker *ExclusionTracker, linterID, ruleID string) *pkg.BoolRule {
	return NewTrackedBoolRuleForModule(disable, tracker, linterID, ruleID, "")
}
