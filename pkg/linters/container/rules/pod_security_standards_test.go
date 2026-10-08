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
	"sort"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/pss"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

// pssGood passes every check of both standards; each case below breaks one thing.
const pssGood = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: app, namespace: d8-test}
spec:
  template:
    metadata:
      labels: {app: app}
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 64535
        runAsGroup: 64535
        seccompProfile: {type: RuntimeDefault}
      containers:
      - name: app
        image: app
        securityContext:
          allowPrivilegeEscalation: false
          capabilities: {drop: [ALL]}
`

const (
	speLabel = `{spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception: spe}}}}}`

	privilegedApp = `{spec: {template: {spec: {containers: [{name: app, securityContext: {privileged: true}}]}}}}`

	hostPathVolume = `
spec:
  template:
    spec:
      volumes: [{name: logs, hostPath: {path: /var/log}}]
      containers: [{name: app, volumeMounts: [{name: logs, mountPath: /logs, readOnly: true}]}]`

	hostNetworkWithPort = `
spec:
  template:
    spec:
      hostNetwork: true
      containers: [{name: app, ports: [{containerPort: 8080, protocol: TCP}]}]`
)

const (
	bD8HostNetwork = "baseline/D8HostNetwork"
	bPrivileged    = "baseline/D8PrivilegedContainer"
	bCapabilities  = "baseline/D8AllowedCapabilities"
	bHostPaths     = "baseline/D8AllowedHostPaths"
	bSeccomp       = "baseline/D8AllowedSeccompProfiles"
	rCapabilities  = "restricted/D8AllowedCapabilities"
	rEscalation    = "restricted/D8AllowPrivilegeEscalation"
	rVolumeTypes   = "restricted/D8AllowedVolumeTypes"
	rUsers         = "restricted/D8AllowedUsers"
	rSeccomp       = "restricted/D8AllowedSeccompProfiles"
)

// Pod security edge cases from the spec of the rule: each is checked both as a
// violation and, where an exception may close it, as covered by a SecurityPolicyException.
func TestPodSecurityStandards_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		patches []string          // strategic merge patches applied to pssGood in order
		spes    map[string]string // SPE name in d8-test -> spec
		want    []string          // fired checks, "standard/Kind"
	}{
		{name: "good pod passes every check"},

		// 1. Container fields: per-container label first, then the common one. Pod fields: common only.
		{
			name:    "1: per-container label binds an SPE",
			patches: []string{privilegedApp, `{spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception.container.app: spe}}}}}`},
			spes:    map[string]string{"spe": `{securityContext: {privileged: {allowedValue: true}}}`},
		},
		{
			name: "1: per-container label wins over the common one",
			patches: []string{privilegedApp, `{spec: {template: {metadata: {labels: {
				security.deckhouse.io/security-policy-exception: spe,
				security.deckhouse.io/security-policy-exception.container.app: narrow}}}}}`},
			spes: map[string]string{
				"spe":    `{securityContext: {privileged: {allowedValue: true}}}`,
				"narrow": `{securityContext: {allowPrivilegeEscalation: {allowedValue: true}}}`,
			},
			want: []string{bPrivileged},
		},
		{
			name: "1: pod field ignores per-container label",
			patches: []string{
				`{spec: {template: {spec: {hostNetwork: true}}}}`,
				`{spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception.container.app: spe}}}}}`,
			},
			spes: map[string]string{"spe": `{network: {hostNetwork: {allowedValue: true}}}`},
			want: []string{bD8HostNetwork},
		},
		{
			name:    "1: pod field is covered via the common label",
			patches: []string{`{spec: {template: {spec: {hostNetwork: true}}}}`, speLabel},
			spes:    map[string]string{"spe": `{network: {hostNetwork: {allowedValue: true}}}`},
		},

		// 2. Controller labels come from the pod template only.
		{
			name:    "2: SPE label on the controller's own metadata is not used",
			patches: []string{privilegedApp, `{metadata: {labels: {security.deckhouse.io/security-policy-exception: spe}}}`},
			spes:    map[string]string{"spe": `{securityContext: {privileged: {allowedValue: true}}}`},
			want:    []string{bPrivileged},
		},
		{
			name:    "2: SPE label on the pod template is used",
			patches: []string{privilegedApp, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {privileged: {allowedValue: true}}}`},
		},

		// 3. containers, initContainers and ephemeralContainers are all checked.
		{
			name:    "3: initContainers are checked",
			patches: []string{`{spec: {template: {spec: {initContainers: [{name: init, image: app, securityContext: {privileged: true, allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}}]}}}}`},
			want:    []string{bPrivileged},
		},
		{
			name:    "3: ephemeralContainers are checked",
			patches: []string{`{spec: {template: {spec: {ephemeralContainers: [{name: debug, image: app, securityContext: {privileged: true, allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}}]}}}}`},
			want:    []string{bPrivileged},
		},
		{
			name: "3: initContainer covered by its per-container label",
			patches: []string{
				`{spec: {template: {spec: {initContainers: [{name: init, image: app, securityContext: {privileged: true, allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}}]}}}}`,
				`{spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception.container.init: spe}}}}}`,
			},
			spes: map[string]string{"spe": `{securityContext: {privileged: {allowedValue: true}}}`},
		},

		// 4. Neither runAsUser nor runAsNonRoot: an SPE does not help, runAsUser must be set.
		{
			name:    "4: no runAsUser and no runAsNonRoot",
			patches: []string{`{spec: {template: {spec: {securityContext: {runAsUser: null, runAsNonRoot: null}}}}}`},
			want:    []string{rUsers},
		},
		{
			name:    "4: no runAsUser and no runAsNonRoot, SPE does not help",
			patches: []string{`{spec: {template: {spec: {securityContext: {runAsUser: null, runAsNonRoot: null}}}}}`, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {runAsUser: {allowedValues: [0]}, runAsNonRoot: {allowedValue: false}}}`},
			want:    []string{rUsers},
		},

		// 5. runAsUser: 0 needs both runAsUser.allowedValues and runAsNonRoot.allowedValue.
		{
			name:    "5: runAsUser 0",
			patches: []string{`{spec: {template: {spec: {securityContext: {runAsUser: 0, runAsNonRoot: null}}}}}`},
			want:    []string{rUsers},
		},
		{
			name:    "5: runAsUser 0, SPE allows runAsUser only",
			patches: []string{`{spec: {template: {spec: {securityContext: {runAsUser: 0, runAsNonRoot: null}}}}}`, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {runAsUser: {allowedValues: [0]}}}`},
			want:    []string{rUsers},
		},
		{
			name:    "5: runAsUser 0, SPE allows runAsNonRoot only",
			patches: []string{`{spec: {template: {spec: {securityContext: {runAsUser: 0, runAsNonRoot: null}}}}}`, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {runAsNonRoot: {allowedValue: false}}}`},
			want:    []string{rUsers},
		},
		{
			name:    "5: runAsUser 0, SPE allows both",
			patches: []string{`{spec: {template: {spec: {securityContext: {runAsUser: 0, runAsNonRoot: null}}}}}`, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {runAsUser: {allowedValues: [0]}, runAsNonRoot: {allowedValue: false}}}`},
		},

		// 6. allowPrivilegeEscalation unset counts as true.
		{
			name:    "6: allowPrivilegeEscalation unset",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {allowPrivilegeEscalation: null}}]}}}}`},
			want:    []string{rEscalation},
		},
		{
			name:    "6: allowPrivilegeEscalation unset, covered by SPE",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {allowPrivilegeEscalation: null}}]}}}}`, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {allowPrivilegeEscalation: {allowedValue: true}}}`},
		},

		// 7. capabilities: drop [ALL] is required, add allows NET_BIND_SERVICE only.
		{
			name:    "7: drop ALL missing",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {capabilities: null}}]}}}}`},
			want:    []string{rCapabilities},
		},
		{
			name:    "7: add NET_BIND_SERVICE",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {capabilities: {add: [NET_BIND_SERVICE]}}}]}}}}`},
		},
		{
			name:    "7: add NET_ADMIN",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {capabilities: {add: [NET_ADMIN]}}}]}}}}`},
			want:    []string{bCapabilities, rCapabilities},
		},
		{
			name:    "7: add NET_ADMIN, covered by SPE",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {capabilities: {add: [NET_ADMIN]}}}]}}}}`, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {capabilities: {allowedValues: {add: [NET_ADMIN], drop: [ALL]}}}}`},
		},

		// 8. hostPath: exact path, strict readOnly match, both volumes.types and volumes.hostPath.
		{
			name:    "8: hostPath",
			patches: []string{hostPathVolume},
			want:    []string{bHostPaths, rVolumeTypes},
		},
		{
			name:    "8: hostPath, covered by SPE",
			patches: []string{hostPathVolume, speLabel},
			spes:    map[string]string{"spe": `{volumes: {types: {allowedValues: [hostPath]}, hostPath: {allowedValues: [{path: /var/log, readOnly: true}]}}}`},
		},
		{
			name:    "8: hostPath, SPE allows the volume type only",
			patches: []string{hostPathVolume, speLabel},
			spes:    map[string]string{"spe": `{volumes: {types: {allowedValues: [hostPath]}}}`},
			want:    []string{bHostPaths},
		},
		{
			name:    "8: hostPath, SPE allows the path only",
			patches: []string{hostPathVolume, speLabel},
			spes:    map[string]string{"spe": `{volumes: {hostPath: {allowedValues: [{path: /var/log, readOnly: true}]}}}`},
			want:    []string{rVolumeTypes},
		},
		{
			name:    "8: hostPath, SPE readOnly differs from the mount",
			patches: []string{hostPathVolume, speLabel},
			spes:    map[string]string{"spe": `{volumes: {types: {allowedValues: [hostPath]}, hostPath: {allowedValues: [{path: /var/log, readOnly: false}]}}}`},
			want:    []string{bHostPaths},
		},
		{
			name:    "8: hostPath, SPE path is a prefix, not the exact path",
			patches: []string{hostPathVolume, speLabel},
			spes:    map[string]string{"spe": `{volumes: {types: {allowedValues: [hostPath]}, hostPath: {allowedValues: [{path: /var, readOnly: true}]}}}`},
			want:    []string{bHostPaths},
		},

		// 9. seccomp: container field, then pod field, then annotations; RuntimeDefault == runtime/default.
		{
			name:    "9: container Unconfined overrides pod RuntimeDefault",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {seccompProfile: {type: Unconfined}}}]}}}}`},
			want:    []string{bSeccomp, rSeccomp},
		},
		{
			name:    "9: container Unconfined, covered by SPE",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {seccompProfile: {type: Unconfined}}}]}}}}`, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {seccompProfile: {allowedValues: [Unconfined]}}}`},
		},
		{
			name:    "9: pod field Unconfined",
			patches: []string{`{spec: {template: {spec: {securityContext: {seccompProfile: {type: Unconfined}}}}}}`},
			want:    []string{bSeccomp, rSeccomp},
		},
		{
			name: "9: annotation runtime/default is a synonym of RuntimeDefault",
			patches: []string{
				`{spec: {template: {spec: {securityContext: {seccompProfile: null}}}}}`,
				`{spec: {template: {metadata: {annotations: {seccomp.security.alpha.kubernetes.io/pod: runtime/default}}}}}`,
			},
		},
		{
			name: "9: annotation unconfined",
			patches: []string{
				`{spec: {template: {spec: {securityContext: {seccompProfile: null}}}}}`,
				`{spec: {template: {metadata: {annotations: {container.seccomp.security.alpha.kubernetes.io/app: unconfined}}}}}`,
			},
			want: []string{bSeccomp, rSeccomp},
		},
		{
			// SPE values are compared as written, synonyms are expanded for parameters only.
			name: "9: annotation unconfined, covered by SPE unconfined",
			patches: []string{
				`{spec: {template: {spec: {securityContext: {seccompProfile: null}}}}}`,
				`{spec: {template: {metadata: {annotations: {container.seccomp.security.alpha.kubernetes.io/app: unconfined}}}}}`,
				speLabel,
			},
			spes: map[string]string{"spe": `{securityContext: {seccompProfile: {allowedValues: [unconfined]}}}`},
		},
		{
			name:    "9: container field wins over the pod annotation",
			patches: []string{`{spec: {template: {metadata: {annotations: {seccomp.security.alpha.kubernetes.io/pod: unconfined}}}}}`, `{spec: {template: {spec: {containers: [{name: app, securityContext: {seccompProfile: {type: RuntimeDefault}}}]}}}}`},
		},

		// 10. hostPort: ranges [] forbids any; hostNetwork makes every containerPort a host port;
		// an SPE hostPorts list allows exactly its {port, protocol} pairs.
		{
			name:    "10: hostPort",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, ports: [{containerPort: 8080, hostPort: 8080, protocol: TCP}]}]}}}}`},
			want:    []string{bD8HostNetwork},
		},
		{
			name:    "10: hostPort, covered by SPE hostPorts",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, ports: [{containerPort: 8080, hostPort: 8080, protocol: TCP}]}]}}}}`, speLabel},
			spes:    map[string]string{"spe": `{network: {hostPorts: [{port: 8080, protocol: TCP}]}}`},
		},
		{
			name:    "10: hostNetwork, containerPort counts as host port",
			patches: []string{hostNetworkWithPort, speLabel},
			spes:    map[string]string{"spe": `{network: {hostNetwork: {allowedValue: true}}}`},
			want:    []string{bD8HostNetwork},
		},
		{
			name:    "10: hostNetwork, containerPort covered by SPE hostPorts",
			patches: []string{hostNetworkWithPort, speLabel},
			spes:    map[string]string{"spe": `{network: {hostNetwork: {allowedValue: true}, hostPorts: [{port: 8080, protocol: TCP}]}}`},
		},
		{
			name:    "10: hostNetwork, SPE hostPorts protocol differs",
			patches: []string{hostNetworkWithPort, speLabel},
			spes:    map[string]string{"spe": `{network: {hostNetwork: {allowedValue: true}, hostPorts: [{port: 8080, protocol: UDP}]}}`},
			want:    []string{bD8HostNetwork},
		},
		{
			name:    "10: hostNetwork, SPE hostPorts lists another port",
			patches: []string{hostNetworkWithPort, speLabel},
			spes:    map[string]string{"spe": `{network: {hostNetwork: {allowedValue: true}, hostPorts: [{port: 9090, protocol: TCP}]}}`},
			want:    []string{bD8HostNetwork},
		},

		// 11. privileged passes all restricted checks, only baseline D8PrivilegedContainer catches it.
		{
			name:    "11: privileged is caught by baseline only",
			patches: []string{privilegedApp},
			want:    []string{bPrivileged},
		},
		{
			name:    "11: privileged, covered by SPE",
			patches: []string{privilegedApp, speLabel},
			spes:    map[string]string{"spe": `{securityContext: {privileged: {allowedValue: true}}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := patchedDeployment(t, tt.patches...)

			spes := make([]map[string]any, 0, len(tt.spes))
			for name, spec := range tt.spes {
				spes = append(spes, spe(t, name, spec))
			}

			assert.ElementsMatch(t, tt.want, fired(t, podOf(obj), pss.Inventory(spes)))
		})
	}
}

// The rego skips a controller whose template lacks runAsUser/runAsNonRoot (a mutator
// might set it). The rule checks the Pod built from the template, so it is caught.
func TestPodSecurityStandards_ControllerStrict(t *testing.T) {
	obj := patchedDeployment(t, `{spec: {template: {spec: {securityContext: {runAsUser: null, runAsNonRoot: null}}}}}`)

	assert.Empty(t, fired(t, obj.Unstructured.Object, pss.Inventory(nil)), "the controller itself passes in lenient mode")
	assert.Equal(t, []string{rUsers}, fired(t, podOf(obj), pss.Inventory(nil)))
}

func TestPodSecurityStandards_PodOf(t *testing.T) {
	cronJob := object(t, `
apiVersion: batch/v1
kind: CronJob
metadata: {name: cron, namespace: d8-test, labels: {top: "true"}}
spec:
  jobTemplate:
    spec:
      template:
        metadata: {labels: {app: cron}, annotations: {a: b}}
        spec:
          containers: [{name: app, securityContext: {privileged: true}}]
`)
	assert.Equal(t, map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "cron", "namespace": "d8-test", "labels": map[string]any{"app": "cron"}, "annotations": map[string]any{"a": "b"}},
		"spec":       map[string]any{"containers": []any{map[string]any{"name": "app", "securityContext": map[string]any{"privileged": true}}}},
	}, podOf(cronJob))
	assert.NotContains(t, cronJob.Unstructured.Object["spec"].(map[string]any)["jobTemplate"].(map[string]any)["spec"].(map[string]any)["template"].(map[string]any)["metadata"], "name", "source object must not be modified")

	pod := object(t, `{apiVersion: v1, kind: Pod, metadata: {name: p, namespace: d8-test}, spec: {containers: [{name: app}]}}`)
	assert.Equal(t, pod.Unstructured.Object, podOf(pod))

	assert.Nil(t, podOf(object(t, `{apiVersion: v1, kind: Service, metadata: {name: s}}`)))
}

func TestPodSecurityStandardsRule(t *testing.T) {
	bad := patchedDeployment(t, privilegedApp, `{spec: {template: {spec: {securityContext: {runAsUser: null, runAsNonRoot: null}}}}}`)

	tests := []struct {
		name    string
		objects []storage.StoreObject
		want    int
	}{
		{name: "one error per object, however many violations", objects: []storage.StoreObject{bad}, want: 1},
		{name: "kube-* is checked", objects: []storage.StoreObject{withNamespace(t, bad, "kube-test")}, want: 1},
		{name: "outside d8-*/kube-* is ignored", objects: []storage.StoreObject{withNamespace(t, bad, "default")}},
		{
			name:    "SPE rendered by the module is used",
			objects: []storage.StoreObject{patchedDeployment(t, privilegedApp, speLabel), {Unstructured: unstructured.Unstructured{Object: spe(t, "spe", `{securityContext: {privileged: {allowedValue: true}}}`)}}},
		},
		{
			name:    "SPE from another namespace is not used",
			objects: []storage.StoreObject{patchedDeployment(t, privilegedApp, speLabel), withNamespace(t, storage.StoreObject{Unstructured: unstructured.Unstructured{Object: spe(t, "spe", `{securityContext: {privileged: {allowedValue: true}}}`)}}, "d8-other")},
			want:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := map[storage.ResourceIndex]storage.StoreObject{}
			for _, o := range tt.objects {
				objects[storage.GetResourceIndex(o)] = o
			}

			m := mocks.NewModuleMock(minimock.NewController(t))
			m.GetStorageMock.Return(objects)

			errorList := errors.NewLintRuleErrorsList()
			NewPodSecurityStandardsRule(m, errorList).Check(t.Context())

			errs := errorList.GetErrors()
			require.Len(t, errs, tt.want)

			for _, e := range errs {
				assert.Equal(t, PodSecurityStandardsRuleName, e.RuleID)
				assert.Contains(t, e.Text, "Deployment/app violates Pod Security Standards (restricted)")
			}
		})
	}
}

func fired(t *testing.T, pod, inventory map[string]any) []string {
	t.Helper()

	violations, err := pss.Eval(t.Context(), pod, inventory)
	require.NoError(t, err)

	res := []string{}

	for _, v := range violations {
		id := v.Standard + "/" + v.Kind
		if len(res) == 0 || res[len(res)-1] != id {
			res = append(res, id)
		}
	}

	sort.Strings(res)

	return res
}

func object(t *testing.T, doc string) storage.StoreObject {
	t.Helper()

	var obj map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))

	return storage.StoreObject{Unstructured: unstructured.Unstructured{Object: obj}}
}

func patchedDeployment(t *testing.T, patches ...string) storage.StoreObject {
	t.Helper()

	doc, err := yaml.YAMLToJSON([]byte(pssGood))
	require.NoError(t, err)

	for _, p := range patches {
		patch, err := yaml.YAMLToJSON([]byte(p))
		require.NoError(t, err)

		doc, err = strategicpatch.StrategicMergePatch(doc, patch, appsv1.Deployment{})
		require.NoError(t, err, p)
	}

	return object(t, string(doc))
}

func withNamespace(t *testing.T, o storage.StoreObject, ns string) storage.StoreObject {
	t.Helper()

	c := storage.StoreObject{Unstructured: *o.Unstructured.DeepCopy()}
	c.Unstructured.SetNamespace(ns)

	return c
}

func spe(t *testing.T, name, spec string) map[string]any {
	t.Helper()

	var s map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(spec), &s))

	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "SecurityPolicyException",
		"metadata":   map[string]any{"name": name, "namespace": "d8-test"},
		"spec":       s,
	}
}

// Violations of several containers in one message, sorted, a msg both standards give
// once; hostNetwork gets the full host port list.
func TestPodSecurityStandardsRule_Message(t *testing.T) {
	obj := patchedDeployment(t, `
spec:
  template:
    spec:
      hostNetwork: true
      initContainers:
      - name: init
        image: init
        ports: [{containerPort: 4224}]
        securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}
      containers:
      - name: app
        securityContext: {privileged: true}
        ports: [{containerPort: 53, protocol: UDP}, {containerPort: 53, protocol: TCP}]
      - name: sidecar
        image: sidecar
        securityContext: {capabilities: {add: [NET_ADMIN]}}`)

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{storage.GetResourceIndex(obj): obj})

	errorList := errors.NewLintRuleErrorsList()
	NewPodSecurityStandardsRule(m, errorList).Check(t.Context())

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Equal(t, `Deployment/app violates Pod Security Standards (restricted) and no SecurityPolicyException covers it:
- D8AllowPrivilegeEscalation: Privilege escalation container is not allowed, container: sidecar | allowPrivilegeEscalation: true | policy allows: false
- D8AllowedCapabilities: container has a disallowed capability, container: sidecar | capabilities.add: ["NET_ADMIN"] | policy allows: ["AUDIT_WRITE", "CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "KILL", "MKNOD", "NET_BIND_SERVICE", "SETFCAP", "SETGID", "SETPCAP", "SETUID", "SYS_CHROOT"]
- D8AllowedCapabilities: container has a disallowed capability, container: sidecar | capabilities.add: ["NET_ADMIN"] | policy allows: ["NET_BIND_SERVICE"]
- D8AllowedCapabilities: container is not dropping all required capabilities, container: sidecar | capabilities.drop: [] | policy allows: ["ALL"]
- D8HostNetwork: The hostNetwork or hostPort are not allowed, Pod: app | hostNetwork: true | policy allows: false
- D8HostNetwork: host ports of the pod (computed by dmt): 53/TCP, 53/UDP, 4224/TCP
- D8PrivilegedContainer: Privileged container is not allowed, container: app | privileged: true | policy allows: false
Fix the pod spec, or, if the deviation is really needed, describe it in a SecurityPolicyException: https://deckhouse.ru/modules/admission-policy-engine/latest/#исключения-из-политик-безопасности`, errs[0].Text)
}
