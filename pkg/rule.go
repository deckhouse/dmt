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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
)

// ExclusionTracker interface for tracking exclusion usage
type ExclusionTracker interface {
	MarkExclusionUsed(linterID, ruleID, exclusion string)
}

// RuleWithTracking provides common tracking functionality
type RuleWithTracking struct {
	tracker  ExclusionTracker
	linterID string
	ruleID   string
}

// SetTracker sets the exclusion tracker for this rule
func (r *RuleWithTracking) SetTracker(tracker ExclusionTracker, linterID, ruleID string) {
	r.tracker = tracker
	r.linterID = linterID
	r.ruleID = ruleID
}

// MarkExclusionUsed marks an exclusion as used if tracker is available
func (r *RuleWithTracking) MarkExclusionUsed(exclusion string) {
	if r.tracker != nil {
		r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, exclusion)
	}
}

type RuleMeta struct {
	Name string
}

func (m *RuleMeta) GetName() string {
	return m.Name
}

type BoolRule struct {
	Exclude bool
	RuleWithTracking
}

func (r *BoolRule) Enabled() bool {
	if r.Exclude {
		r.MarkExclusionUsed("disabled")
	}
	return !r.Exclude
}

type StringRule struct {
	ExcludeRules []StringRuleExclude
	RuleWithTracking
}

func (r *StringRule) Enabled(str string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(str) {
			r.MarkExclusionUsed(string(rule))
			return false
		}
	}

	return true
}

type PrefixRule struct {
	ExcludeRules []PrefixRuleExclude
	RuleWithTracking
}

func (r *PrefixRule) Enabled(str string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(str) {
			r.MarkExclusionUsed(string(rule))
			return false
		}
	}

	return true
}

type KindRule struct {
	ExcludeRules []KindRuleExclude
	RuleWithTracking
}

func (r *KindRule) Enabled(kind, name string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(kind, name) {
			exclusionKey := fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
			r.MarkExclusionUsed(exclusionKey)
			return false
		}
	}

	return true
}

type ContainerRule struct {
	ExcludeRules []ContainerRuleExclude
	RuleWithTracking
}

func (r *ContainerRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(object, container) {
			var exclusionKey string
			if rule.Container == "" {
				exclusionKey = fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
			} else {
				exclusionKey = fmt.Sprintf("%s/%s/%s", rule.Kind, rule.Name, rule.Container)
			}
			r.MarkExclusionUsed(exclusionKey)
			return false
		}
	}

	return true
}

type StringRuleExclude string

func (e StringRuleExclude) Enabled(str string) bool {
	return string(e) != str
}

type PrefixRuleExclude string

func (e PrefixRuleExclude) Enabled(str string) bool {
	return !strings.HasPrefix(str, string(e))
}

type ServicePortRule struct {
	ExcludeRules []ServicePortExclude
	RuleWithTracking
}

func (r *ServicePortRule) Enabled(name, port string) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(name, port) {
			exclusionKey := fmt.Sprintf("%s:%s", rule.Name, rule.Port)
			r.MarkExclusionUsed(exclusionKey)
			return false
		}
	}

	return true
}

type ServicePortExclude struct {
	Name string
	Port string
}

func (e *ServicePortExclude) Enabled(name, port string) bool {
	if e.Name == name &&
		e.Port == port {
		return false
	}

	return true
}

type KindRuleExclude struct {
	Kind string
	Name string
}

func (e *KindRuleExclude) Enabled(kind, name string) bool {
	if e.Kind == kind &&
		e.Name == name {
		return false
	}

	return true
}

type ContainerRuleExclude struct {
	Kind      string
	Name      string
	Container string
}

func (e *ContainerRuleExclude) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	if e.Kind == object.Unstructured.GetKind() &&
		e.Name == object.Unstructured.GetName() &&
		(e.Container == "" || e.Container == container.Name) {
		return false
	}

	return true
}

type PathRule struct {
	ExcludeStringRules []StringRuleExclude
	ExcludePrefixRules []PrefixRuleExclude
	RuleWithTracking
}

func (r *PathRule) Enabled(name string) bool {
	for _, rule := range r.ExcludeStringRules {
		if !rule.Enabled(name) {
			r.MarkExclusionUsed(string(rule))
			return false
		}
	}

	for _, rule := range r.ExcludePrefixRules {
		if !rule.Enabled(name) {
			r.MarkExclusionUsed(string(rule))
			return false
		}
	}

	return true
}

// Helper functions for creating rules with tracking

// NewStringRule creates a new StringRule without tracking
func NewStringRule(excludeRules []StringRuleExclude) *StringRule {
	return &StringRule{
		ExcludeRules: excludeRules,
	}
}

// NewStringRuleWithTracker creates a new StringRule with optional tracking
func NewStringRuleWithTracker(excludeRules []StringRuleExclude, tracker ExclusionTracker, linterID, ruleID string) *StringRule {
	rule := &StringRule{
		ExcludeRules: excludeRules,
	}
	if tracker != nil {
		rule.SetTracker(tracker, linterID, ruleID)
	}
	return rule
}

// NewPrefixRule creates a new PrefixRule without tracking
func NewPrefixRule(excludeRules []PrefixRuleExclude) *PrefixRule {
	return &PrefixRule{
		ExcludeRules: excludeRules,
	}
}

// NewPrefixRuleWithTracker creates a new PrefixRule with optional tracking
func NewPrefixRuleWithTracker(excludeRules []PrefixRuleExclude, tracker ExclusionTracker, linterID, ruleID string) *PrefixRule {
	rule := &PrefixRule{
		ExcludeRules: excludeRules,
	}
	if tracker != nil {
		rule.SetTracker(tracker, linterID, ruleID)
	}
	return rule
}

// NewKindRule creates a new KindRule without tracking
func NewKindRule(excludeRules []KindRuleExclude) *KindRule {
	return &KindRule{
		ExcludeRules: excludeRules,
	}
}

// NewKindRuleWithTracker creates a new KindRule with optional tracking
func NewKindRuleWithTracker(excludeRules []KindRuleExclude, tracker ExclusionTracker, linterID, ruleID string) *KindRule {
	rule := &KindRule{
		ExcludeRules: excludeRules,
	}
	if tracker != nil {
		rule.SetTracker(tracker, linterID, ruleID)
	}
	return rule
}

// NewContainerRule creates a new ContainerRule without tracking
func NewContainerRule(excludeRules []ContainerRuleExclude) *ContainerRule {
	return &ContainerRule{
		ExcludeRules: excludeRules,
	}
}

// NewContainerRuleWithTracker creates a new ContainerRule with optional tracking
func NewContainerRuleWithTracker(excludeRules []ContainerRuleExclude, tracker ExclusionTracker, linterID, ruleID string) *ContainerRule {
	rule := &ContainerRule{
		ExcludeRules: excludeRules,
	}
	if tracker != nil {
		rule.SetTracker(tracker, linterID, ruleID)
	}
	return rule
}

// NewServicePortRule creates a new ServicePortRule without tracking
func NewServicePortRule(excludeRules []ServicePortExclude) *ServicePortRule {
	return &ServicePortRule{
		ExcludeRules: excludeRules,
	}
}

// NewServicePortRuleWithTracker creates a new ServicePortRule with optional tracking
func NewServicePortRuleWithTracker(excludeRules []ServicePortExclude, tracker ExclusionTracker, linterID, ruleID string) *ServicePortRule {
	rule := &ServicePortRule{
		ExcludeRules: excludeRules,
	}
	if tracker != nil {
		rule.SetTracker(tracker, linterID, ruleID)
	}
	return rule
}

// NewPathRule creates a new PathRule without tracking
func NewPathRule(excludeStringRules []StringRuleExclude, excludePrefixRules []PrefixRuleExclude) *PathRule {
	return &PathRule{
		ExcludeStringRules: excludeStringRules,
		ExcludePrefixRules: excludePrefixRules,
	}
}

// NewPathRuleWithTracker creates a new PathRule with optional tracking
func NewPathRuleWithTracker(excludeStringRules []StringRuleExclude, excludePrefixRules []PrefixRuleExclude, tracker ExclusionTracker, linterID, ruleID string) *PathRule {
	rule := &PathRule{
		ExcludeStringRules: excludeStringRules,
		ExcludePrefixRules: excludePrefixRules,
	}
	if tracker != nil {
		rule.SetTracker(tracker, linterID, ruleID)
	}
	return rule
}

// NewBoolRule creates a new BoolRule without tracking
func NewBoolRule(disable bool) *BoolRule {
	return &BoolRule{
		Exclude: disable,
	}
}

// NewBoolRuleWithTracker creates a new BoolRule with optional tracking
func NewBoolRuleWithTracker(disable bool, tracker ExclusionTracker, linterID, ruleID string) *BoolRule {
	rule := &BoolRule{
		Exclude: disable,
	}
	if tracker != nil {
		rule.SetTracker(tracker, linterID, ruleID)
	}
	return rule
}
