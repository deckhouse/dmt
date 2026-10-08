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
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestUndescribedAllowances(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want []string
	}{
		{name: "empty spec"},
		{
			name: "described allowances",
			spec: `
securityContext:
  privileged: {allowedValue: true, metadata: {description: needs devices}}
  runAsUser: {allowedValues: [0], metadata: {description: root}}
  capabilities: {allowedValues: {add: [NET_ADMIN]}, metadata: {description: netlink}}
network:
  hostNetwork: {allowedValue: true, metadata: {description: node agent}}
volumes:
  types: {allowedValues: [hostPath], metadata: {description: host files}}`,
		},
		{
			name: "missing, empty, whitespace and TODO descriptions",
			spec: `
securityContext:
  privileged: {allowedValue: true}
  runAsUser: {allowedValues: [0], metadata: {description: ""}}
  runAsNonRoot: {allowedValue: false, metadata: {description: "  \n"}}
  seccompProfile: {allowedValues: [Unconfined], metadata: {description: " todo "}}
  readOnlyRootFilesystem: {allowedValue: false, metadata: {description: 42}}
network:
  hostPID: {allowedValue: true, metadata: {}}`,
			want: []string{
				"spec.network.hostPID",
				"spec.securityContext.privileged",
				"spec.securityContext.readOnlyRootFilesystem",
				"spec.securityContext.runAsNonRoot",
				"spec.securityContext.runAsUser",
				"spec.securityContext.seccompProfile",
			},
		},
		{
			name: "allowedValues items are values, not allowances",
			spec: `
securityContext:
  sysctls: {allowedValues: [{name: net.ipv4.ip_forward, value: "1"}], metadata: {description: routing}}
  seLinuxOptions: {allowedValues: [{type: spc_t}]}`,
			want: []string{"spec.securityContext.seLinuxOptions"},
		},
		{
			name: "node without allowedValue(s) is not checked",
			spec: `{securityContext: {foo: {bar: baz}}, network: {}}`,
		},
		{
			name: "hostPath: description per allowedValues item",
			spec: `
volumes:
  hostPath:
    allowedValues:
    - {path: /proc, readOnly: true, metadata: {description: procfs}}
    - {path: /sys, readOnly: true}
    - {path: /dev, readOnly: false, metadata: {description: TODO}}`,
			want: []string{"spec.volumes.hostPath.allowedValues[1]", "spec.volumes.hostPath.allowedValues[2]"},
		},
		{
			name: "hostPorts: description per item",
			spec: `
network:
  hostPorts:
  - {port: 53, protocol: UDP, metadata: {description: dns}}
  - {port: 53, protocol: TCP}`,
			want: []string{"spec.network.hostPorts[1]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var spec map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(tt.spec), &spec))
			assert.Equal(t, tt.want, undescribedAllowances(spec))
		})
	}
}

func TestSecurityPolicyExceptionDescriptionRule(t *testing.T) {
	objects := []storage.StoreObject{
		speObject(t, "a", "d8-test", `{securityContext: {privileged: {allowedValue: true}, runAsUser: {allowedValues: [0]}}}`),
		speObject(t, "b", "default", `{network: {hostIPC: {allowedValue: true}}}`),
	}

	errs := runRule(t, objects, func(m pkg.Module, l *errors.LintRuleErrorsList) pkg.Rule {
		return NewSecurityPolicyExceptionDescriptionRule(m, l)
	})

	texts := make([]string, 0, len(errs))
	for _, e := range errs {
		assert.Equal(t, SecurityPolicyExceptionDescriptionRuleName, e.RuleID)
		texts = append(texts, e.Text)
	}

	assert.ElementsMatch(t, []string{
		"SecurityPolicyException d8-test/a: allowance spec.securityContext.privileged has no metadata.description",
		"SecurityPolicyException d8-test/a: allowance spec.securityContext.runAsUser has no metadata.description",
		"SecurityPolicyException default/b: allowance spec.network.hostIPC has no metadata.description",
	}, texts)
}

func TestSecurityPolicyExceptionUnusedRule(t *testing.T) {
	const allowance = `{securityContext: {privileged: {allowedValue: true, metadata: {description: d}}}}`

	tests := []struct {
		name    string
		objects []storage.StoreObject
		unused  bool
	}{
		{
			name:    "referenced by the common label",
			objects: []storage.StoreObject{patchedDeployment(t, speLabel)},
		},
		{
			name:    "referenced by a container label",
			objects: []storage.StoreObject{patchedDeployment(t, `{spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception.container.app: spe}}}}}`)},
		},
		{
			name: "referenced from a CronJob job template",
			objects: []storage.StoreObject{object(t, `
apiVersion: batch/v1
kind: CronJob
metadata: {name: cron, namespace: d8-test}
spec: {jobTemplate: {spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception: spe}}}}}}`)},
		},
		{
			name:    "referenced by a Pod",
			objects: []storage.StoreObject{object(t, `{apiVersion: v1, kind: Pod, metadata: {name: p, namespace: d8-test, labels: {security.deckhouse.io/security-policy-exception: spe}}}`)},
		},
		{
			name:    "reference from another namespace does not count",
			objects: []storage.StoreObject{withNamespace(t, patchedDeployment(t, speLabel), "d8-other")},
			unused:  true,
		},
		{
			name:    "controller labels do not count, only the pod template",
			objects: []storage.StoreObject{patchedDeployment(t, `{metadata: {labels: {security.deckhouse.io/security-policy-exception: spe}}}`)},
			unused:  true,
		},
		{
			name:    "label of another SPE",
			objects: []storage.StoreObject{patchedDeployment(t, `{spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception: other}}}}}`)},
			unused:  true,
		},
		{
			name:   "no references at all",
			unused: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := append([]storage.StoreObject{speObject(t, "spe", "d8-test", allowance)}, tt.objects...)

			errs := runRule(t, objects, func(m pkg.Module, l *errors.LintRuleErrorsList) pkg.Rule {
				return NewSecurityPolicyExceptionUnusedRule(nil, m, l)
			})

			if !tt.unused {
				assert.Empty(t, errs)

				return
			}

			require.Len(t, errs, 1)
			assert.Equal(t, SecurityPolicyExceptionUnusedRuleName, errs[0].RuleID)
			assert.Contains(t, errs[0].Text, "SecurityPolicyException d8-test/spe is not referenced")
		})
	}
}

func TestSecurityPolicyExceptionUnusedRuleExclude(t *testing.T) {
	const allowance = `{securityContext: {privileged: {allowedValue: true, metadata: {description: d}}}}`

	objects := []storage.StoreObject{
		speObject(t, "excluded", "d8-test", allowance),
		speObject(t, "kept", "d8-test", allowance),
	}

	errs := runRule(t, objects, func(m pkg.Module, l *errors.LintRuleErrorsList) pkg.Rule {
		return NewSecurityPolicyExceptionUnusedRule([]pkg.StringRuleExclude{"excluded"}, m, l)
	})

	levels := map[string]pkg.Level{}
	for _, e := range errs {
		levels[e.ObjectID] = e.Level
	}

	assert.Equal(t, map[string]pkg.Level{
		objects[0].Identity(): pkg.Ignored,
		objects[1].Identity(): pkg.Error,
	}, levels)
}

func runRule(t *testing.T, objects []storage.StoreObject, newRule func(pkg.Module, *errors.LintRuleErrorsList) pkg.Rule) []pkg.LinterError {
	t.Helper()

	byIndex := map[storage.ResourceIndex]storage.StoreObject{}
	for _, o := range objects {
		byIndex[storage.GetResourceIndex(o)] = o
	}

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetStorageMock.Return(byIndex)

	errorList := errors.NewLintRuleErrorsList()
	newRule(m, errorList).Check(t.Context())

	return errorList.GetErrors()
}

func speObject(t *testing.T, name, ns, spec string) storage.StoreObject {
	t.Helper()

	return withNamespace(t, storage.StoreObject{Unstructured: unstructured.Unstructured{Object: spe(t, name, spec)}}, ns)
}
