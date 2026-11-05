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

package rules

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	MonitorSourceLabelRuleName = "monitor-source-label"

	// monitorSourceLabelName and monitorSourceLabelValue are the label every
	// Deckhouse-managed scrape attaches to the series it collects, so that system
	// rules and dashboards can tell them apart from same-named user metrics.
	monitorSourceLabelName  = "d8_source"
	monitorSourceLabelValue = "dkp"

	prometheusOperatorGroup = "monitoring.coreos.com"
)

// MonitorSourceLabelRule enforces that every scrape a module declares labels the
// series it collects with d8_source="dkp", and that every recording rule the module
// ships carries the same label on the series it records.
//
// It looks only at where series get the label, never at PromQL expressions:
//   - PodMonitor and ServiceMonitor: each endpoint's relabelings;
//   - ScrapeConfig: spec.relabelings;
//   - Probe: the relabelingConfigs of its static and ingress targets;
//   - PrometheusRule: the labels of each recording rule.
//
// A relabeling counts when it sets targetLabel d8_source to replacement dkp with
// the default (replace) action. Objects are excluded by kind and name.
type MonitorSourceLabelRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*MonitorSourceLabelRule)(nil)

func NewMonitorSourceLabelRule(excludeRules []pkg.KindRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *MonitorSourceLabelRule {
	return &MonitorSourceLabelRule{
		RuleMeta: pkg.RuleMeta{
			Name: MonitorSourceLabelRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(MonitorSourceLabelRuleName),
	}
}

func (r *MonitorSourceLabelRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		if !strings.HasPrefix(object.Unstructured.GetAPIVersion(), prometheusOperatorGroup+"/") {
			continue
		}

		if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
			continue
		}

		r.checkObject(object)
	}
}

func (r *MonitorSourceLabelRule) checkObject(object storage.StoreObject) {
	errorList := r.errorList.WithObjectID(object.Identity()).WithFilePath(object.GetPath())
	obj := object.Unstructured.Object

	switch object.Unstructured.GetKind() {
	case "PodMonitor":
		checkMonitorEndpoints(obj, "podMetricsEndpoints", errorList)
	case "ServiceMonitor":
		checkMonitorEndpoints(obj, "endpoints", errorList)
	case "ScrapeConfig":
		relabelings, _, _ := unstructured.NestedSlice(obj, "spec", "relabelings")
		if !hasSourceRelabeling(relabelings) {
			errorList.Errorf("spec.relabelings must set %s", sourceRelabelingText())
		}
	case "Probe":
		checkProbeTargets(obj, errorList)
	case "PrometheusRule":
		checkRecordingRuleLabels(obj, errorList)
	}
}

func checkMonitorEndpoints(obj map[string]any, field string, errorList *errors.LintRuleErrorsList) {
	endpoints, _, _ := unstructured.NestedSlice(obj, "spec", field)

	for i, ie := range endpoints {
		endpoint, ok := ie.(map[string]any)
		if !ok {
			continue
		}

		relabelings, _, _ := unstructured.NestedSlice(endpoint, "relabelings")
		if !hasSourceRelabeling(relabelings) {
			errorList.Errorf("spec.%s[%d] (%s) must set %s in its relabelings",
				field, i, describeEndpoint(endpoint), sourceRelabelingText())
		}
	}
}

func checkProbeTargets(obj map[string]any, errorList *errors.LintRuleErrorsList) {
	for _, target := range []string{"staticConfig", "ingress"} {
		if _, found, _ := unstructured.NestedMap(obj, "spec", "targets", target); !found {
			continue
		}

		relabelings, _, _ := unstructured.NestedSlice(obj, "spec", "targets", target, "relabelingConfigs")
		if !hasSourceRelabeling(relabelings) {
			errorList.Errorf("spec.targets.%s.relabelingConfigs must set %s", target, sourceRelabelingText())
		}
	}
}

func checkRecordingRuleLabels(obj map[string]any, errorList *errors.LintRuleErrorsList) {
	groups, _, _ := unstructured.NestedSlice(obj, "spec", "groups")

	for _, ig := range groups {
		group, ok := ig.(map[string]any)
		if !ok {
			continue
		}

		groupName, _, _ := unstructured.NestedString(group, "name")
		rules, _, _ := unstructured.NestedSlice(group, "rules")

		for _, ir := range rules {
			rule, ok := ir.(map[string]any)
			if !ok {
				continue
			}

			record, _, _ := unstructured.NestedString(rule, "record")
			if record == "" {
				continue
			}

			value, _, _ := unstructured.NestedString(rule, "labels", monitorSourceLabelName)
			if value != monitorSourceLabelValue {
				errorList.Errorf("recording rule '%s' (group '%s') must set label %s: %s",
					record, groupName, monitorSourceLabelName, monitorSourceLabelValue)
			}
		}
	}
}

// hasSourceRelabeling reports whether one of the relabelings sets the source label.
// Prometheus-operator CRDs spell the fields in camelCase; the action defaults to
// replace and is case-insensitive.
func hasSourceRelabeling(relabelings []any) bool {
	for _, ir := range relabelings {
		relabeling, ok := ir.(map[string]any)
		if !ok {
			continue
		}

		targetLabel, _, _ := unstructured.NestedString(relabeling, "targetLabel")
		replacement, _, _ := unstructured.NestedString(relabeling, "replacement")
		action, _, _ := unstructured.NestedString(relabeling, "action")

		if targetLabel == monitorSourceLabelName && replacement == monitorSourceLabelValue &&
			(action == "" || strings.EqualFold(action, "replace")) {
			return true
		}
	}

	return false
}

func describeEndpoint(endpoint map[string]any) string {
	var parts []string

	for _, key := range []string{"port", "targetPort", "path"} {
		if v, ok := endpoint[key]; ok {
			parts = append(parts, fmt.Sprintf("%s %v", key, v))
		}
	}

	if len(parts) == 0 {
		return "endpoint"
	}

	return strings.Join(parts, ", ")
}

func sourceRelabelingText() string {
	return fmt.Sprintf("a relabeling with targetLabel: %s and replacement: %s", monitorSourceLabelName, monitorSourceLabelValue)
}
