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

package rules

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	AlertGroupingAnnotationsRuleName = "alert-grouping-annotations"

	// The suffix after the prefix only ties a "create group" annotation to its
	// "grouped by" counterpart; the group name is the annotation value.
	AnnotationCreateGroupPrefix = "plk_create_group_if_not_exists__"
	AnnotationGroupedByPrefix   = "plk_grouped_by__"

	prometheusRulesDir = "monitoring/prometheus-rules"
)

func NewAlertGroupingAnnotationsRule(excludeRules []pkg.StringRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *AlertGroupingAnnotationsRule {
	return &AlertGroupingAnnotationsRule{
		RuleMeta: pkg.RuleMeta{
			Name: AlertGroupingAnnotationsRuleName,
		},
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(AlertGroupingAnnotationsRuleName),
	}
}

type AlertGroupingAnnotationsRule struct {
	pkg.RuleMeta
	pkg.StringRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*AlertGroupingAnnotationsRule)(nil)

// collisionKey dedupes a collision seen both in a source file and in a rendered object.
type collisionKey struct {
	alert      string
	annotation string
}

// groupNameFromAnnotation takes the group name out of a value like
// "D8RegistryGroup,tier=cluster": the first comma-separated element, the rest being
// label matchers.
func groupNameFromAnnotation(value string) string {
	name, _, _ := strings.Cut(value, ",")

	return strings.TrimSpace(name)
}

// namesMayCollide reports whether an alert name and a group name can denote the same
// trigger.
//
// Files are read as written, so a templated name is seen as its source text, and a
// literal comparison would miss the collision that only appears after substitution:
// "{{ $controllerKind }}ImageAbsent" is the alert "DeploymentImageAbsent" once
// rendered for that kind. When exactly one side is templated it becomes a pattern
// matched against the other; otherwise the texts are compared, since both sides
// render in the same context.
//
// templateActionRe comes from the openapi-values-quote rule in this package and
// already handles the `{{-` / `-}}` trim markers.
func namesMayCollide(alertName, groupName string) bool {
	alertTemplated := templateActionRe.MatchString(alertName)
	groupTemplated := templateActionRe.MatchString(groupName)

	if alertTemplated == groupTemplated {
		return alertName == groupName
	}

	pattern, literal := alertName, groupName
	if groupTemplated {
		pattern, literal = groupName, alertName
	}

	// A name of nothing but actions says nothing about what it renders to: its
	// pattern "^.+$" would match every group. Treat that as no evidence.
	if strings.TrimSpace(templateActionRe.ReplaceAllString(pattern, "")) == "" {
		return false
	}

	return templateNamePattern(pattern).MatchString(literal)
}

// templateNamePattern turns a templated name into an anchored pattern, with every
// template action standing for the one or more characters it will render to.
func templateNamePattern(name string) *regexp.Regexp {
	var b strings.Builder

	b.WriteString("^")

	last := 0
	for _, loc := range templateActionRe.FindAllStringIndex(name, -1) {
		b.WriteString(regexp.QuoteMeta(name[last:loc[0]]))
		b.WriteString(".+")

		last = loc[1]
	}

	b.WriteString(regexp.QuoteMeta(name[last:]))
	b.WriteString("$")

	re, err := regexp.Compile(b.String())
	if err != nil {
		// Unreachable, every literal segment is quoted. Refuse to match rather
		// than report on a pattern that was not understood.
		return regexp.MustCompile(`\A\z(?:x)`)
	}

	return re
}

func isGroupingAnnotation(key string) bool {
	return strings.HasPrefix(key, AnnotationCreateGroupPrefix) ||
		strings.HasPrefix(key, AnnotationGroupedByPrefix)
}

func (r *AlertGroupingAnnotationsRule) Check(_ context.Context) {
	seen := make(map[collisionKey]struct{})

	// Files first, and they are the load-bearing half: PrometheusRule objects render
	// only when global.enabledModules has "operator-prometheus-crd", which a full
	// deckhouse lint does not set, so an object-only check would silently pass.
	r.checkSourceFiles(seen)
	r.checkRenderedObjects(seen)
}

// checkSourceFiles walks monitoring/prometheus-rules and checks the rule files as
// written, independently of whether the module renders them into objects.
func (r *AlertGroupingAnnotationsRule) checkSourceFiles(seen map[collisionKey]struct{}) {
	modulePath := r.module.GetPath()
	if modulePath == "" {
		return
	}

	rulesDir := filepath.Join(modulePath, filepath.FromSlash(prometheusRulesDir))
	if info, err := os.Stat(rulesDir); err != nil || !info.IsDir() {
		return
	}

	_ = filepath.WalkDir(rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable entry is not this rule's concern
		}

		// helm_lib globs "**.{yaml,tpl}" here, so template files carry rules too.
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml", ".tpl":
			r.checkRuleFile(path, seen)
		}

		return nil
	})
}

func (r *AlertGroupingAnnotationsRule) checkRuleFile(path string, seen map[collisionKey]struct{}) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil || len(doc.Content) == 0 {
		// Template files are usually not valid YAML on their own, which rules out
		// the structured walk; a line scan keeps them covered rather than skipped.
		r.checkRuleFileLines(content, path, seen)

		return
	}

	for _, group := range groupNodes(doc.Content[0]) {
		for _, alertRule := range sequenceItems(mappingValue(group, "rules")) {
			r.checkAlertNode(alertRule, path, seen)
		}
	}
}

// groupNodes returns a file's rule groups, accepting all three shapes a module may
// ship: a bare list of groups (the one modules actually use), a "groups" mapping, and
// a whole PrometheusRule manifest.
func groupNodes(root *yaml.Node) []*yaml.Node {
	if root == nil {
		return nil
	}

	if root.Kind == yaml.SequenceNode {
		return root.Content
	}

	if groups := mappingValue(root, "groups"); groups != nil {
		return sequenceItems(groups)
	}

	return sequenceItems(mappingValue(mappingValue(root, "spec"), "groups"))
}

func (r *AlertGroupingAnnotationsRule) checkAlertNode(
	alertRule *yaml.Node,
	path string,
	seen map[collisionKey]struct{},
) {
	// Recording rules carry no "alert" field and are never grouped.
	alertNode := mappingValue(alertRule, "alert")
	if alertNode == nil || alertNode.Kind != yaml.ScalarNode || alertNode.Value == "" {
		return
	}

	alertName := alertNode.Value
	if !r.Enabled(alertName) {
		return
	}

	annotations := mappingValue(alertRule, "annotations")
	if annotations == nil || annotations.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i+1 < len(annotations.Content); i += 2 {
		key, value := annotations.Content[i], annotations.Content[i+1]
		if value.Kind != yaml.ScalarNode || !isGroupingAnnotation(key.Value) {
			continue
		}

		if !namesMayCollide(alertName, groupNameFromAnnotation(value.Value)) {
			continue
		}

		r.report(alertName, key.Value, path, "", value.Line, seen)
	}
}

// checkRenderedObjects covers modules that build PrometheusRule objects some way
// other than from monitoring/prometheus-rules.
func (r *AlertGroupingAnnotationsRule) checkRenderedObjects(seen map[collisionKey]struct{}) {
	for _, object := range r.module.GetStorage() {
		if object.Unstructured.GetKind() != "PrometheusRule" {
			continue
		}

		groups, found, err := unstructured.NestedSlice(object.Unstructured.Object, "spec", "groups")
		if err != nil || !found {
			continue
		}

		for _, group := range groups {
			groupMap, ok := group.(map[string]any)
			if !ok {
				continue
			}

			r.checkRenderedGroup(groupMap, object.Identity(), object.GetPath(), seen)
		}
	}
}

func (r *AlertGroupingAnnotationsRule) checkRenderedGroup(
	group map[string]any,
	objectID, filePath string,
	seen map[collisionKey]struct{},
) {
	promRules, found, err := unstructured.NestedSlice(group, "rules")
	if err != nil || !found {
		return
	}

	for _, promRule := range promRules {
		ruleMap, ok := promRule.(map[string]any)
		if !ok {
			continue
		}

		alertName, found, err := unstructured.NestedString(ruleMap, "alert")
		if err != nil || !found || alertName == "" {
			continue
		}

		if !r.Enabled(alertName) {
			continue
		}

		annotations, found, err := unstructured.NestedStringMap(ruleMap, "annotations")
		if err != nil || !found {
			continue
		}

		for key, value := range annotations {
			if !isGroupingAnnotation(key) || !namesMayCollide(alertName, groupNameFromAnnotation(value)) {
				continue
			}

			r.report(alertName, key, filePath, objectID, 0, seen)
		}
	}
}

func (r *AlertGroupingAnnotationsRule) report(
	alertName, annotation, filePath, objectID string,
	line int,
	seen map[collisionKey]struct{},
) {
	key := collisionKey{alert: alertName, annotation: annotation}
	if _, reported := seen[key]; reported {
		return
	}

	seen[key] = struct{}{}

	errorList := r.errorList
	if objectID != "" {
		errorList = errorList.WithObjectID(objectID)
	}

	errorList = errorList.WithFilePath(filePath)
	if line > 0 {
		errorList = errorList.WithLineNumber(line)
	}

	errorList.Errorf(
		"Alert %q is grouped into itself: annotation %q names %q as the group. "+
			"An alert cannot be its own group — it is a circular dependency, and the alert is dropped "+
			"instead of being delivered. Name the group differently from the alert, e.g. %q",
		alertName, annotation, alertName, alertName+"Group",
	)
}

// alertLineRe and groupingAnnotationLineRe drive the line scan. Both are anchored so
// that an alert name quoted in a description is not read as a declaration.
//
// The name is the whole rest of the line, not one token: template files build names
// out of values, and matching one token would fail on those lines and leave the
// previous alert's name in hand, attributing what follows to the wrong alert.
var (
	alertLineRe              = regexp.MustCompile(`^\s*-\s*alert:\s*(.+?)\s*$`)
	groupingAnnotationLineRe = regexp.MustCompile(
		`^\s*(plk_(?:create_group_if_not_exists|grouped_by)__[^:\s]+):\s*(.+?)\s*$`)
)

// checkRuleFileLines ties each grouping annotation to the nearest preceding
// "- alert:" line. Weaker than the YAML walk, which sees nesting, but for these files
// the alternative is no check at all.
func (r *AlertGroupingAnnotationsRule) checkRuleFileLines(
	content []byte,
	path string,
	seen map[collisionKey]struct{},
) {
	var alertName string

	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()

		if match := alertLineRe.FindStringSubmatch(text); match != nil {
			alertName = strings.Trim(match[1], `"'`)

			continue
		}

		if alertName == "" {
			continue
		}

		match := groupingAnnotationLineRe.FindStringSubmatch(text)
		if match == nil {
			continue
		}

		if !namesMayCollide(alertName, groupNameFromAnnotation(strings.Trim(match[2], `"'`))) {
			continue
		}

		if !r.Enabled(alertName) {
			continue
		}

		r.report(alertName, match[1], path, "", line, seen)
	}
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}

	return nil
}

func sequenceItems(node *yaml.Node) []*yaml.Node {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}

	return node.Content
}
