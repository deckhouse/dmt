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
	"os"
	"path/filepath"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

	// An empty module path keeps the source-file half of the rule a no-op, so these
	// cases exercise the rendered-object half alone.
	return runAlertGroupingRuleAt(t, t.TempDir(), store, excludes)
}

func runAlertGroupingRuleAt(
	t *testing.T,
	modulePath string,
	store map[storage.ResourceIndex]storage.StoreObject,
	excludes []pkg.StringRuleExclude,
) *errors.LintRuleErrorsList {
	t.Helper()

	mc := minimock.NewController(t)

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(store)
	mod.GetPathMock.Return(modulePath)

	errorList := errors.NewLintRuleErrorsList()
	NewAlertGroupingAnnotationsRule(excludes, mod, errorList).Check(t.Context())

	return errorList
}

// writeRuleFile drops a prometheus rules file into a module tree, in the same
// monitoring/prometheus-rules layout deckhouse modules use.
func writeRuleFile(t *testing.T, modulePath, name, content string) {
	t.Helper()

	dir := filepath.Join(modulePath, "monitoring", "prometheus-rules")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
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

// --- source-file scanning -------------------------------------------------
//
// These matter most: on a full deckhouse lint no PrometheusRule object is rendered
// at all (rendering is gated behind "operator-prometheus-crd" in global.enabledModules),
// so the rule has to read the files to catch anything.

const selfGroupingRuleFile = `- name: d8.registry
  rules:
    - alert: D8RegistryDrainStuck
      expr: vector(1)
      annotations:
        plk_protocol_version: "1"
        plk_create_group_if_not_exists__d8_registry_drain_stuck: "D8RegistryDrainStuck,tier=cluster"
        plk_grouped_by__d8_registry_drain_stuck: "D8RegistryDrainStuck,tier=cluster"
        summary: The registry module cannot finish leaving the pull path.
`

func TestAlertGroupingAnnotations_SourceFile_Collision(t *testing.T) {
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.yaml", selfGroupingRuleFile)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.True(t, errorList.ContainsErrors())
	assert.Len(t, errorList.GetErrors(), 2)
	assert.Contains(t, errorList.GetErrors()[0].Text, "D8RegistryDrainStuck")
}

func TestAlertGroupingAnnotations_SourceFile_ReportsLineNumbers(t *testing.T) {
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.yaml", selfGroupingRuleFile)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	lines := make([]int, 0, 2)
	for _, e := range errorList.GetErrors() {
		lines = append(lines, e.LineNumber)
	}

	assert.ElementsMatch(t, []int{7, 8}, lines)
}

func TestAlertGroupingAnnotations_SourceFile_DistinctGroup(t *testing.T) {
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.yaml", `- name: d8.registry
  rules:
    - alert: D8RegistryDrainStuck
      expr: vector(1)
      annotations:
        plk_create_group_if_not_exists__d8_registry_alerts: "D8RegistryAlerts,tier=cluster"
        plk_grouped_by__d8_registry_alerts: "D8RegistryAlerts,tier=cluster"
`)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_SourceFile_WholeManifestShape(t *testing.T) {
	// A file holding a full PrometheusRule manifest rather than a bare group list.
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "rules.yaml", `apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: registry
spec:
  groups:
    - name: d8.registry
      rules:
        - alert: D8RegistryDrainStuck
          expr: vector(1)
          annotations:
            plk_grouped_by__d8_registry_drain_stuck: "D8RegistryDrainStuck,tier=cluster"
`)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.True(t, errorList.ContainsErrors())
	assert.Len(t, errorList.GetErrors(), 1)
}

func TestAlertGroupingAnnotations_SourceFile_UnparsableFileWithoutAlertsIsQuiet(t *testing.T) {
	// Broken YAML falls through to the line scan, which finds no alert declaration
	// here and so reports nothing. Complaining about the syntax is promtool's job.
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "broken.yaml", "- name: d8.registry\n  rules: [oops\n")

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.False(t, errorList.ContainsErrors())
}

// --- template rule files --------------------------------------------------
//
// helm_lib globs "**.{yaml,tpl}", and most .tpl rule files are not valid YAML on
// their own, so these go through the line-scan fallback rather than the YAML walk.

const templateRuleFile = `{{- if .Values.global.enabledModules }}
- name: d8.registry
  rules:
    - alert: D8RegistryDrainStuck
      expr: vector(1)
      annotations:
        plk_grouped_by__d8_registry_drain_stuck: "D8RegistryDrainStuck,tier=cluster"
        summary: {{ $labels.node }} is stuck
{{- end }}
`

func TestAlertGroupingAnnotations_TemplateFile_CollisionFound(t *testing.T) {
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.tpl", templateRuleFile)

	// Guard the premise: this content really is not parsable as YAML, so the test
	// exercises the fallback and not the structured walk.
	var probe yaml.Node
	require.Error(t, yaml.Unmarshal([]byte(templateRuleFile), &probe))

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.True(t, errorList.ContainsErrors())
	assert.Len(t, errorList.GetErrors(), 1)
	assert.Equal(t, 7, errorList.GetErrors()[0].LineNumber)
}

func TestAlertGroupingAnnotations_TemplateFile_DistinctGroup(t *testing.T) {
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.tpl", `{{- if true }}
- name: d8.registry
  rules:
    - alert: D8RegistryDrainStuck
      annotations:
        plk_grouped_by__d8_registry_alerts: "D8RegistryAlerts,tier=cluster"
{{- end }}
`)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_TemplateFile_AlertNameInProseIgnored(t *testing.T) {
	// A description mentioning the alert name must not be read as a declaration.
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.tpl", `{{- if true }}
- name: d8.registry
  rules:
    - alert: D8RegistryOther
      annotations:
        description: see alert: D8RegistryDrainStuck for details
        plk_grouped_by__d8_registry_alerts: "D8RegistryAlerts,tier=cluster"
{{- end }}
`)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_SourceFile_NoRulesDirectory(t *testing.T) {
	errorList := runAlertGroupingRuleAt(t, t.TempDir(), nil, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_SourceFile_ExcludedAlert(t *testing.T) {
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.yaml", selfGroupingRuleFile)

	excludes := pkg.StringRuleExcludeList{"D8RegistryDrainStuck"}.Get()
	errorList := runAlertGroupingRuleAt(t, modulePath, nil, excludes)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_FileAndObjectReportedOnce(t *testing.T) {
	// The same alert reaching the rule through both halves must not be reported twice.
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "drain.yaml", selfGroupingRuleFile)

	store := makePrometheusRuleStorage(alertRule{
		name: "D8RegistryDrainStuck",
		annotations: map[string]string{
			"plk_create_group_if_not_exists__d8_registry_drain_stuck": "D8RegistryDrainStuck,tier=cluster",
			"plk_grouped_by__d8_registry_drain_stuck":                 "D8RegistryDrainStuck,tier=cluster",
		},
	})

	errorList := runAlertGroupingRuleAt(t, modulePath, store, nil)

	assert.Len(t, errorList.GetErrors(), 2)
}

func TestAlertGroupingAnnotations_TemplateFile_TemplatedNameNotAttributedToPreviousAlert(t *testing.T) {
	// A templated alert name must not leave the previous alert's name in hand: the
	// annotations below belong to the templated alert, not to D8RegistryDrainStuck.
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "mixed.tpl", `{{- if true }}
- name: d8.registry
  rules:
    - alert: D8RegistryDrainStuck
      annotations:
        plk_grouped_by__d8_registry_alerts: "D8RegistryAlerts,tier=cluster"
    - alert: {{ $controllerKind }}ImageAbsent
      annotations:
        plk_grouped_by__drain: "D8RegistryDrainStuck,tier=cluster"
{{- end }}
`)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.False(t, errorList.ContainsErrors())
}

func TestAlertGroupingAnnotations_TemplateFile_TemplatedNameCollision(t *testing.T) {
	// When a templated name really does group into itself, the texts are identical
	// and the collision is still caught.
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "templated.tpl", `{{- if true }}
- name: d8.images
  rules:
    - alert: {{ $controllerKind }}ImageAbsent
      annotations:
        plk_grouped_by__images: "{{ $controllerKind }}ImageAbsent,tier=cluster"
{{- end }}
`)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.True(t, errorList.ContainsErrors())
	assert.Len(t, errorList.GetErrors(), 1)
}

// --- имена, собираемые шаблоном -------------------------------------------

func TestNamesMayCollide(t *testing.T) {
	cases := []struct {
		name   string
		alert  string
		group  string
		expect bool
	}{
		{
			name:   "plain names, equal",
			alert:  "D8RegistryDrainStuck",
			group:  "D8RegistryDrainStuck",
			expect: true,
		},
		{
			name:   "plain names, different",
			alert:  "D8RegistryDrainStuck",
			group:  "D8RegistryAlerts",
			expect: false,
		},
		{
			name:   "templated alert, group is what it renders to",
			alert:  "{{ $controllerKind }}ImageAbsent",
			group:  "DeploymentImageAbsent",
			expect: true,
		},
		{
			name:   "templated alert, unrelated group — the real extended-monitoring case",
			alert:  "{{ $controllerKind }}ImageAbsent",
			group:  "UnavailableImagesInNamespace",
			expect: false,
		},
		{
			name:   "templated alert, group shares only the prefix",
			alert:  "{{ $controllerKind }}ImageAbsent",
			group:  "ImageAbsentSomethingElse",
			expect: false,
		},
		{
			name:   "templated group, literal alert",
			alert:  "DeploymentImageAbsent",
			group:  "{{ $controllerKind }}ImageAbsent",
			expect: true,
		},
		{
			name:   "both templated and identical — same context, same trigger",
			alert:  "{{ $controllerKind }}ImageAbsent",
			group:  "{{ $controllerKind }}ImageAbsent",
			expect: true,
		},
		{
			name:   "both templated and different",
			alert:  "{{ $controllerKind }}ImageAbsent",
			group:  "{{ $controllerKind }}Group",
			expect: false,
		},
		{
			name:   "trim markers are handled",
			alert:  "{{- $controllerKind -}}ImageAbsent",
			group:  "DaemonSetImageAbsent",
			expect: true,
		},
		{
			name:   "the action must stand for at least one character",
			alert:  "{{ $controllerKind }}ImageAbsent",
			group:  "ImageAbsent",
			expect: false,
		},
		{
			name:   "a name made only of actions matches nothing, not everything",
			alert:  "{{ $alertName }}",
			group:  "D8RegistryAlerts",
			expect: false,
		},
		{
			name:   "same, with the templated side being the group",
			alert:  "D8RegistryDrainStuck",
			group:  "{{ $groupName }}",
			expect: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expect, namesMayCollide(c.alert, c.group))
		})
	}
}

func TestAlertGroupingAnnotations_TemplateFile_CollisionOnlyAfterSubstitution(t *testing.T) {
	// The group names one concrete expansion of the templated alert, so at render
	// time that alert groups into itself. Reading the file literally would miss it.
	modulePath := t.TempDir()
	writeRuleFile(t, modulePath, "images.tpl", `{{- define "by-kind" }}
- alert: {{ $controllerKind }}ImageAbsent
  annotations:
    plk_grouped_by__images: "DeploymentImageAbsent,tier=cluster"
{{- end }}
`)

	errorList := runAlertGroupingRuleAt(t, modulePath, nil, nil)

	assert.True(t, errorList.ContainsErrors())
	assert.Len(t, errorList.GetErrors(), 1)
}
