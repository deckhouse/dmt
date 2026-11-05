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
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func sourceRelabeling() map[string]any {
	return map[string]any{"targetLabel": "d8_source", "replacement": "dkp"}
}

func tierRelabeling() map[string]any {
	return map[string]any{"targetLabel": "tier", "replacement": "cluster"}
}

func monitoringObject(kind, name string, spec map[string]any) storage.StoreObject {
	return storage.StoreObject{Unstructured: unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "monitoring.coreos.com/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": "d8-monitoring"},
		"spec":       spec,
	}}}
}

func endpointWith(port string, relabelings ...map[string]any) map[string]any {
	items := make([]any, 0, len(relabelings))
	for _, r := range relabelings {
		items = append(items, r)
	}

	return map[string]any{"port": port, "relabelings": items}
}

func runMonitorSourceLabelRule(t *testing.T, exclude []pkg.KindRuleExclude, objects ...storage.StoreObject) *errors.LintRuleErrorsList {
	t.Helper()

	mc := minimock.NewController(t)
	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storeFrom(objects...))

	errorList := errors.NewLintRuleErrorsList()
	NewMonitorSourceLabelRule(exclude, mod, errorList).Check(t.Context())

	return errorList
}

func TestMonitorSourceLabelRule_PodMonitor(t *testing.T) {
	labeled := monitoringObject("PodMonitor", "ok", map[string]any{
		"podMetricsEndpoints": []any{endpointWith("https-metrics", tierRelabeling(), sourceRelabeling())},
	})
	assert.False(t, runMonitorSourceLabelRule(t, nil, labeled).ContainsErrors())

	// One endpoint of two is enough to fail: each endpoint is a scrape of its own.
	partial := monitoringObject("PodMonitor", "partial", map[string]any{
		"podMetricsEndpoints": []any{
			endpointWith("self", sourceRelabeling()),
			endpointWith("hooks", tierRelabeling()),
		},
	})
	errs := runMonitorSourceLabelRule(t, nil, partial).GetErrors()
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, "spec.podMetricsEndpoints[1] (port hooks)")

	withoutRelabelings := monitoringObject("PodMonitor", "bare", map[string]any{
		"podMetricsEndpoints": []any{map[string]any{"port": "metrics"}},
	})
	assert.True(t, runMonitorSourceLabelRule(t, nil, withoutRelabelings).ContainsErrors())
}

func TestMonitorSourceLabelRule_ServiceMonitor(t *testing.T) {
	labeled := monitoringObject("ServiceMonitor", "ok", map[string]any{
		"endpoints": []any{endpointWith("http", sourceRelabeling())},
	})
	assert.False(t, runMonitorSourceLabelRule(t, nil, labeled).ContainsErrors())

	unlabeled := monitoringObject("ServiceMonitor", "missing", map[string]any{
		"endpoints": []any{endpointWith("http", tierRelabeling())},
	})
	assert.True(t, runMonitorSourceLabelRule(t, nil, unlabeled).ContainsErrors())
}

func TestMonitorSourceLabelRule_RelabelingMustSetTheExactLabel(t *testing.T) {
	for name, relabeling := range map[string]map[string]any{
		"legacy source label": {"targetLabel": "source", "replacement": "deckhouse"},
		"other value":         {"targetLabel": "d8_source", "replacement": "user"},
		"non-replace action":  {"targetLabel": "d8_source", "replacement": "dkp", "action": "hashmod"},
	} {
		t.Run(name, func(t *testing.T) {
			obj := monitoringObject("PodMonitor", "pm", map[string]any{
				"podMetricsEndpoints": []any{endpointWith("metrics", relabeling)},
			})
			assert.True(t, runMonitorSourceLabelRule(t, nil, obj).ContainsErrors())
		})
	}

	explicitReplace := monitoringObject("PodMonitor", "pm", map[string]any{
		"podMetricsEndpoints": []any{endpointWith("metrics",
			map[string]any{"action": "Replace", "targetLabel": "d8_source", "replacement": "dkp"})},
	})
	assert.False(t, runMonitorSourceLabelRule(t, nil, explicitReplace).ContainsErrors())
}

func TestMonitorSourceLabelRule_ScrapeConfig(t *testing.T) {
	labeled := monitoringObject("ScrapeConfig", "ok", map[string]any{"relabelings": []any{sourceRelabeling()}})
	assert.False(t, runMonitorSourceLabelRule(t, nil, labeled).ContainsErrors())

	unlabeled := monitoringObject("ScrapeConfig", "missing", map[string]any{"staticConfigs": []any{}})
	assert.True(t, runMonitorSourceLabelRule(t, nil, unlabeled).ContainsErrors())
}

func TestMonitorSourceLabelRule_Probe(t *testing.T) {
	labeled := monitoringObject("Probe", "ok", map[string]any{
		"targets": map[string]any{"staticConfig": map[string]any{"relabelingConfigs": []any{sourceRelabeling()}}},
	})
	assert.False(t, runMonitorSourceLabelRule(t, nil, labeled).ContainsErrors())

	unlabeled := monitoringObject("Probe", "missing", map[string]any{
		"targets": map[string]any{"ingress": map[string]any{}},
	})
	assert.True(t, runMonitorSourceLabelRule(t, nil, unlabeled).ContainsErrors())
}

func TestMonitorSourceLabelRule_RecordingRules(t *testing.T) {
	rules := monitoringObject("PrometheusRule", "rules", map[string]any{
		"groups": []any{map[string]any{
			"name": "g",
			"rules": []any{
				map[string]any{"record": "d8:labeled", "expr": "sum(a)", "labels": map[string]any{"d8_source": "dkp"}},
				map[string]any{"record": "d8:unlabeled", "expr": "sum(b)"},
				// Alerts are not recordings: their labels are not a scrape label.
				map[string]any{"alert": "SomeAlert", "expr": "c > 0"},
			},
		}},
	})

	errs := runMonitorSourceLabelRule(t, nil, rules).GetErrors()
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, "recording rule 'd8:unlabeled' (group 'g')")
}

func TestMonitorSourceLabelRule_SkipsForeignAndExcludedObjects(t *testing.T) {
	unlabeled := func(kind, name string) storage.StoreObject {
		return monitoringObject(kind, name, map[string]any{
			"podMetricsEndpoints": []any{endpointWith("metrics")},
		})
	}

	// A same-named kind from another API group is not a prometheus-operator monitor.
	foreign := unlabeled("PodMonitor", "foreign")
	foreign.Unstructured.SetAPIVersion("example.com/v1")
	assert.False(t, runMonitorSourceLabelRule(t, nil, foreign).ContainsErrors())

	exclude := []pkg.KindRuleExclude{{Kind: "PodMonitor", Name: "third-party"}}
	assert.False(t, runMonitorSourceLabelRule(t, exclude, unlabeled("PodMonitor", "third-party")).ContainsErrors())
	assert.True(t, runMonitorSourceLabelRule(t, exclude, unlabeled("PodMonitor", "ours")).ContainsErrors())
}
