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
	"context"
	"strings"

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

// groupNameFromAnnotation extracts the group name from a grouping annotation value.
//
// The value is a comma-separated list whose first element is the name of the group
// to group into, the rest being label matchers, e.g.
// "D8RegistryGroup,tier=cluster,prometheus=deckhouse".
func groupNameFromAnnotation(value string) string {
	name, _, _ := strings.Cut(value, ",")

	return strings.TrimSpace(name)
}

func isGroupingAnnotation(key string) bool {
	return strings.HasPrefix(key, AnnotationCreateGroupPrefix) ||
		strings.HasPrefix(key, AnnotationGroupedByPrefix)
}

func (r *AlertGroupingAnnotationsRule) Check(_ context.Context) {
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

			r.checkGroup(groupMap, object.Identity(), object.GetPath())
		}
	}
}

func (r *AlertGroupingAnnotationsRule) checkGroup(group map[string]any, objectID, filePath string) {
	promRules, found, err := unstructured.NestedSlice(group, "rules")
	if err != nil || !found {
		return
	}

	for _, promRule := range promRules {
		ruleMap, ok := promRule.(map[string]any)
		if !ok {
			continue
		}

		// Recording rules carry no "alert" field and are never grouped.
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

		r.checkAlertAnnotations(alertName, annotations, objectID, filePath)
	}
}

func (r *AlertGroupingAnnotationsRule) checkAlertAnnotations(
	alertName string,
	annotations map[string]string,
	objectID, filePath string,
) {
	for key, value := range annotations {
		if !isGroupingAnnotation(key) {
			continue
		}

		if groupNameFromAnnotation(value) != alertName {
			continue
		}

		r.errorList.WithObjectID(objectID).
			WithFilePath(filePath).
			Errorf(
				"Alert %q is grouped into itself: annotation %q names %q as the group. "+
					"An alert cannot be its own group — it is a circular dependency, and the alert is dropped "+
					"instead of being delivered. Name the group differently from the alert, e.g. %q",
				alertName, key, alertName, alertName+"Group",
			)
	}
}
