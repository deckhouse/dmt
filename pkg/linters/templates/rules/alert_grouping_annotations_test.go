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
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// promRuleObjectName names the generated PrometheusRule object; the rule reports it
// as the object ID, and no test depends on the particular value.
const promRuleObjectName = "registry"

// alertRule describes one alerting rule to put into the generated PrometheusRule.
type alertRule struct {
	name        string
	annotations map[string]string
}

func makePrometheusRuleStorage(alerts ...alertRule) map[storage.ResourceIndex]storage.StoreObject {
	promRules := make([]any, 0, len(alerts))

	for _, alert := range alerts {
		rule := map[string]any{"expr": "vector(1)"}
		if alert.name != "" {
			rule["alert"] = alert.name
		}

		if alert.annotations != nil {
			annotations := make(map[string]any, len(alert.annotations))
			for k, v := range alert.annotations {
				annotations[k] = v
			}

			rule["annotations"] = annotations
		}

		promRules = append(promRules, rule)
	}

	u := unstructured.Unstructured{}
	u.SetKind("PrometheusRule")
	u.SetName(promRuleObjectName)
	u.Object["spec"] = map[string]any{
		"groups": []any{
			map[string]any{
				"name":  "test.group",
				"rules": promRules,
			},
		},
	}

	return map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "PrometheusRule", Name: promRuleObjectName}: {
			Unstructured: u,
			AbsPath:      "/test/" + promRuleObjectName + ".yaml",
		},
	}
}

func runAlertGroupingRule(
	t *testing.T,
	store map[storage.ResourceIndex]storage.StoreObject,
	excludes []pkg.StringRuleExclude,
) *errors.LintRuleErrorsList {
	t.Helper()

	mc := minimock.NewController(t)

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(store)

	errorList := errors.NewLintRuleErrorsList()
	NewAlertGroupingAnnotationsRule(excludes, mod, errorList).Check(t.Context())

	return errorList
}

func TestAlertGroupingAnnotations_GroupEqualsAlertName(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_create_group_if_not_exists__d8_registry_drain_stuck": "D8RegistryDrainStuck,tier=cluster,prometheus=deckhouse",
			"plk_grouped_by__d8_registry_drain_stuck":                 "D8RegistryDrainStuck,tier=cluster,prometheus=deckhouse",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.True(t, errorList.ContainsErrors())
	// Both annotations are reported, so neither is silently left behind after a fix.
	assert.Len(t, errorList.GetErrors(), 2)
	assert.Contains(t, errorList.GetErrors()[0].Text, "D8RegistryDrainStuck")
}

func TestAlertGroupingAnnotations_GroupDiffersFromAlertName(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_create_group_if_not_exists__d8_registry_group": "D8RegistryGroup,tier=cluster,prometheus=deckhouse",
			"plk_grouped_by__d8_registry_group":                 "D8RegistryGroup,tier=cluster,prometheus=deckhouse",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_OnlyGroupedByCollides(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_create_group_if_not_exists__d8_registry_group": "D8RegistryGroup,tier=cluster",
			"plk_grouped_by__d8_registry_drain_stuck":           "D8RegistryDrainStuck,tier=cluster",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.True(t, errorList.ContainsErrors())
	assert.Len(t, errorList.GetErrors(), 1)
}

func TestAlertGroupingAnnotations_NoGroupingAnnotations(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"summary":     "Registry drain is stuck",
			"description": "D8RegistryDrainStuck",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_NoAnnotationsAtAll(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{name: "D8RegistryDrainStuck"})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_GroupNameWithoutComma(t *testing.T) {
	// A bare group name with no label matchers is still a group name.
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_grouped_by__d8_registry_drain_stuck": "D8RegistryDrainStuck",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.True(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_GroupNameSurroundedBySpaces(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_grouped_by__d8_registry_drain_stuck": "  D8RegistryDrainStuck , tier=cluster",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.True(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_EmptyAnnotationValue(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_grouped_by__d8_registry_drain_stuck": "",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_DifferentCaseIsADifferentTrigger(t *testing.T) {
	// Trigger names are matched exactly, so a differently-cased group name is a
	// genuinely different group and must not be reported.
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_grouped_by__d8_registry_drain_stuck": "d8registrydrainstuck,tier=cluster",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_RecordingRuleIgnored(t *testing.T) {
	// A recording rule has no "alert" field; its annotations must not be inspected.
	store := makePrometheusRuleStorage(alertRule{
		name: "",
		annotations: map[string]string{
			"plk_grouped_by__whatever": "whatever",
		},
	})

	errorList := runAlertGroupingRule(t, store, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_SkipsNonPrometheusRuleObjects(t *testing.T) {
	u := unstructured.Unstructured{}
	u.SetKind("Deployment")
	u.SetName("my-deploy")
	u.SetAnnotations(map[string]string{
		"plk_grouped_by__my_deploy": "my-deploy",
	})

	store := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "my-deploy"}: {
			Unstructured: u,
			AbsPath:      "/test/deploy.yaml",
		},
	}

	errorList := runAlertGroupingRule(t, store, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_ExcludedAlert(t *testing.T) {
	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_grouped_by__d8_registry_drain_stuck": "D8RegistryDrainStuck,tier=cluster",
		},
	})

	excludes := pkg.StringRuleExcludeList{"D8RegistryDrainStuck"}.Get()
	errorList := runAlertGroupingRule(t, store, excludes)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_ExcludedAlertDoesNotHideOthers(t *testing.T) {
	store := makePrometheusRuleStorage(
		alertRule{
			name: "D8RegistryDrainStuck",
			annotations: map[string]string{
				"plk_grouped_by__d8_registry_drain_stuck": "D8RegistryDrainStuck,tier=cluster",
			},
		},
		alertRule{
			name: "D8RegistryNodeForeignRegistryConfig",
			annotations: map[string]string{
				"plk_grouped_by__d8_registry_node_foreign_registry_config": "D8RegistryNodeForeignRegistryConfig,tier=cluster",
			},
		},
	)

	excludes := pkg.StringRuleExcludeList{"D8RegistryDrainStuck"}.Get()
	errorList := runAlertGroupingRule(t, store, excludes)

	assert.True(t, errorList.ContainsErrors())
	assert.Len(t, errorList.GetErrors(), 1)
	assert.Contains(t, errorList.GetErrors()[0].Text, "D8RegistryNodeForeignRegistryConfig")
}
