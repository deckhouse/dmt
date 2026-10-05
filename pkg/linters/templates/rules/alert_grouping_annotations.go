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

	// Grouping annotations read by the alerts processing system. The part after the
	// prefix is an arbitrary suffix tying a "create group" annotation to its
	// "grouped by" counterpart; the group name itself lives in the annotation value.
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

// collisionKey identifies one reported collision, so that an alert found both in
// its source file and in a rendered object is reported once.
type collisionKey struct {
	alert      string
	annotation string
}

// groupNameFromAnnotation extracts the group name from a grouping annotation value.
//
// The value is a comma-separated list whose first element is the name of the group
// to group into, the rest being label matchers, e.g.
// "D8RegistryGroup,tier=cluster,prometheus=deckhouse".
func groupNameFromAnnotation(value string) string {
	name, _, _ := strings.Cut(value, ",")

	return strings.TrimSpace(name)
}

// namesMayCollide reports whether an alert name and a group name can denote the same
// trigger.
//
// Rule files are read as written, so a name built by a template is seen as its source
// text. Comparing that text literally would miss the collision that only appears after
// substitution: "- alert: {{ $controllerKind }}ImageAbsent" rendered for the Deployment
// kind is the alert "DeploymentImageAbsent", and a group named "DeploymentImageAbsent"
// collides with it. So when exactly one side is templated, the templated one is turned
// into a pattern and matched against the other.
//
// When both sides are templated they are rendered in the same context, so identical
// text means one trigger and different text means two; and when neither is, this is a
// plain comparison.
//
// templateActionRe is the one declared for the openapi-values-quote rule: same package,
// same job, and it already accounts for the `{{-` / `-}}` trim markers.
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
		// Every literal segment is quoted, so this cannot happen; refuse to match
		// rather than report a collision on a pattern that was not understood.
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

	// The source files are checked first, and they are the load-bearing half: rendering
	// a module's PrometheusRule objects is gated behind "operator-prometheus-crd" being
	// in global.enabledModules, so on a full deckhouse lint no such object exists and an
	// object-only check would silently pass. The files are always on disk.
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

		// .tpl is not an afterthought: helm_lib globs "**.{yaml,tpl}" out of this
		// directory, and modules ship 26 template rule files, 15 of them carrying
		// grouping annotations.
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
		// Rule files may be Go templates, and most of those are not valid YAML on
		// their own, so the structured walk is not available for them. Falling back
		// to a line scan keeps them covered instead of silently skipping 16 of the
		// 26 template files deckhouse modules ship.
		r.checkRuleFileLines(content, path, seen)

		return
	}

	for _, group := range groupNodes(doc.Content[0]) {
		for _, alertRule := range sequenceItems(mappingValue(group, "rules")) {
			r.checkAlertNode(alertRule, path, seen)
		}
	}
}

// groupNodes returns the rule groups of a parsed file, accepting the three shapes a
// module may ship: a bare list of groups (what deckhouse modules use), a mapping with
// a "groups" key, and a whole PrometheusRule manifest with "spec.groups".
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

// checkRenderedObjects covers PrometheusRule objects that do not come from
// monitoring/prometheus-rules, for modules that build them some other way.
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

// alertLineRe and groupingAnnotationLineRe drive the line-scan fallback. They are
// anchored at both ends so that prose mentioning an alert name in a description
// cannot be mistaken for a declaration.
//
// The name is captured as the whole rest of the line rather than one token, because
// template files build names out of values — "- alert: {{ $controllerKind }}ImageAbsent"
// is real. Matching a single token would fail on those lines and leave the previous
// alert's name in hand, quietly attributing the annotations that follow to the wrong
// alert. Keeping the raw text also keeps the comparison meaningful: a templated name
// and a templated group name collide exactly when their text is identical.
var (
	alertLineRe              = regexp.MustCompile(`^\s*-\s*alert:\s*(.+?)\s*$`)
	groupingAnnotationLineRe = regexp.MustCompile(
		`^\s*(plk_(?:create_group_if_not_exists|grouped_by)__[^:\s]+):\s*(.+?)\s*$`)
)

// checkRuleFileLines attributes each grouping annotation to the nearest preceding
// "- alert:" line. That is weaker than the YAML walk — it cannot see nesting — but a
// template file keeps its annotations directly under their alert, and the alternative
// for these files is no check at all.
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
