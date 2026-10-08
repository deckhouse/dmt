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
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/pss"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// The skeleton for each edge case: the SPE built from the spec passes the CRD schema
// (proposeSPE validates it) and, with the pod bound to it, the rego fires nothing but
// what no SPE covers.
func TestProposeSPE(t *testing.T) {
	tests := []struct {
		name     string
		patches  []string
		spes     map[string]string // SPEs the module renders, name -> spec
		want     map[string]string // SPE name -> spec additions, YAML
		bind     bool              // the common label is to be added
		leftover []string          // fired with the SPE, "standard/Kind"
	}{
		{
			name: "hostNetwork: every containerPort is a host port, TCP by default",
			patches: []string{`
spec:
  template:
    spec:
      hostNetwork: true
      initContainers: [{name: init, image: init, ports: [{containerPort: 4224}], securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}}]
      containers: [{name: app, ports: [{containerPort: 53, protocol: UDP}, {containerPort: 53, protocol: TCP}]}]`},
			bind: true,
			want: map[string]string{"app": `
network:
  hostNetwork: {allowedValue: true, metadata: {description: TODO}}
  hostPorts:
  - {port: 53, protocol: TCP, metadata: {description: TODO}}
  - {port: 53, protocol: UDP, metadata: {description: TODO}}
  - {port: 4224, protocol: TCP, metadata: {description: TODO}}`},
		},
		{
			name:    "hostPath: exact path, readOnly of the mount, hostPath volume type",
			patches: []string{hostPathVolume},
			bind:    true,
			want: map[string]string{"app": `
volumes:
  hostPath:
    allowedValues:
    - {path: /var/log, readOnly: true, metadata: {description: TODO}}
  types: {allowedValues: [hostPath], metadata: {description: TODO}}`},
		},
		{
			name: "seccomp: the raw value of the annotation",
			patches: []string{
				`{spec: {template: {spec: {securityContext: {seccompProfile: null}}}}}`,
				`{spec: {template: {metadata: {annotations: {container.seccomp.security.alpha.kubernetes.io/app: unconfined}}}}}`,
			},
			bind: true,
			want: map[string]string{"app": `
securityContext:
  seccompProfile: {allowedValues: [unconfined], metadata: {description: TODO}}`},
		},
		{
			name: "seccomp: the raw value of the field",
			patches: []string{
				`{spec: {template: {spec: {containers: [{name: app, securityContext: {seccompProfile: {type: Unconfined}}}]}}}}`,
			},
			bind: true,
			want: map[string]string{"app": `
securityContext:
  seccompProfile: {allowedValues: [Unconfined], metadata: {description: TODO}}`},
		},
		{
			name:    "runAsUser 0: runAsUser and runAsNonRoot",
			patches: []string{`{spec: {template: {spec: {securityContext: {runAsUser: 0, runAsNonRoot: null}}}}}`},
			bind:    true,
			want: map[string]string{"app": `
securityContext:
  runAsNonRoot: {allowedValue: false, metadata: {description: TODO}}
  runAsUser: {allowedValues: [0], metadata: {description: TODO}}`},
		},
		{
			name:    "capabilities: add as it is, drop from the defaults",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {capabilities: {add: [NET_ADMIN, NET_BIND_SERVICE]}}}]}}}}`},
			bind:    true,
			want: map[string]string{"app": `
securityContext:
  capabilities: {allowedValues: {add: [NET_ADMIN, NET_BIND_SERVICE]}, metadata: {description: TODO}}`},
		},
		{
			name:    "capabilities: drop without ALL",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {capabilities: {drop: [NET_RAW, MKNOD]}}}]}}}}`},
			bind:    true,
			want: map[string]string{"app": `
securityContext:
  capabilities: {allowedValues: {drop: [NET_RAW, MKNOD]}, metadata: {description: TODO}}`},
		},
		{
			name:    "privileged: only privileged, escalation is set to false",
			patches: []string{privilegedApp},
			bind:    true,
			want: map[string]string{"app": `
securityContext:
  privileged: {allowedValue: true, metadata: {description: TODO}}`},
		},
		{
			name:    "AppArmor: a field value in the annotation form",
			patches: []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {appArmorProfile: {type: Unconfined}}}]}}}}`},
			bind:    true,
			want: map[string]string{"app": `
securityContext:
  appArmorProfile: {allowedValues: [unconfined], metadata: {description: TODO}}`},
		},
		{
			name: "container label: container fields to its SPE, pod fields to the common one",
			patches: []string{
				privilegedApp,
				`{spec: {template: {spec: {hostPID: true}}}}`,
				`{spec: {template: {metadata: {labels: {security.deckhouse.io/security-policy-exception.container.app: narrow}}}}}`,
			},
			bind: true,
			want: map[string]string{
				"app":    "network:\n  hostPID: {allowedValue: true, metadata: {description: TODO}}",
				"narrow": "securityContext:\n  privileged: {allowedValue: true, metadata: {description: TODO}}",
			},
		},
		{
			name:    "bound SPE: only what it lacks",
			patches: []string{speLabel, privilegedApp, `{spec: {template: {spec: {hostNetwork: true, containers: [{name: app, ports: [{containerPort: 8080}]}]}}}}`},
			spes:    map[string]string{"spe": `{securityContext: {privileged: {allowedValue: true}}, network: {hostNetwork: {allowedValue: true}}}`},
			want: map[string]string{"spe": `
network:
  hostPorts:
  - {port: 8080, protocol: TCP, metadata: {description: TODO}}`},
		},

		// No SPE covers these: the skeleton covers the rest, the leftover is reported.
		{
			name:     "empty drop: add is covered, drop is not",
			patches:  []string{`{spec: {template: {spec: {containers: [{name: app, securityContext: {capabilities: {drop: null, add: [NET_ADMIN]}}}]}}}}`},
			bind:     true,
			want:     map[string]string{"app": "securityContext:\n  capabilities: {allowedValues: {add: [NET_ADMIN]}, metadata: {description: TODO}}"},
			leftover: []string{rCapabilities},
		},
		{
			name: "hostPath mounted read-only and read-write: not in the SPE",
			patches: []string{hostPathVolume, `
spec:
  template:
    spec:
      initContainers: [{name: init, image: init, volumeMounts: [{name: logs, mountPath: /logs}], securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}}]`},
			bind:     true,
			want:     map[string]string{"app": "volumes:\n  types: {allowedValues: [hostPath], metadata: {description: TODO}}"},
			leftover: []string{bHostPaths},
		},
		{
			name: "AppArmor Localhost of the field",
			patches: []string{
				privilegedApp,
				`{spec: {template: {spec: {containers: [{name: app, securityContext: {appArmorProfile: {type: Localhost, localhostProfile: p}}}]}}}}`,
			},
			bind:     true,
			want:     map[string]string{"app": "securityContext:\n  privileged: {allowedValue: true, metadata: {description: TODO}}"},
			leftover: []string{"baseline/D8AppArmor"},
		},
		{
			name: "sysctls",
			patches: []string{
				privilegedApp,
				`{spec: {template: {spec: {securityContext: {sysctls: [{name: kernel.msgmax, value: "65536"}]}}}}}`,
			},
			bind:     true,
			want:     map[string]string{"app": "securityContext:\n  privileged: {allowedValue: true, metadata: {description: TODO}}"},
			leftover: []string{"baseline/D8AllowedSysctls"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := patchedDeployment(t, tt.patches...)
			pod := podOf(obj)

			objects := map[storage.ResourceIndex]storage.StoreObject{}

			spes := make([]map[string]any, 0, len(tt.spes))

			for name, spec := range tt.spes {
				s := spe(t, name, spec)
				o := storage.StoreObject{Unstructured: unstructured.Unstructured{Object: s}}
				objects[storage.GetResourceIndex(o)] = o

				spes = append(spes, s)
			}

			inventory, err := pss.Inventory(spes)
			require.NoError(t, err)

			violations, err := pss.Eval(t.Context(), pod, inventory)
			require.NoError(t, err)
			require.NotEmpty(t, violations)

			p, err := proposeSPE(t.Context(), pod, violations, objects)
			require.NoError(t, err)

			got := map[string]any{}
			for name, add := range p.adds {
				got[name] = add
			}

			want := map[string]any{}

			for name, doc := range tt.want {
				var m map[string]any
				require.NoError(t, yaml.Unmarshal([]byte(doc), &m))
				want[name] = m
			}

			wantYAML, _ := yaml.Marshal(want)
			gotYAML, _ := yaml.Marshal(got)
			assert.Equal(t, string(wantYAML), string(gotYAML))
			assert.Equal(t, tt.bind, p.bindGeneral)

			// Re-eval on its own: the module's SPEs with the additions, the pod bound.
			bound := deepCopy(pod)
			if p.bindGeneral {
				nested(bound, "metadata", "labels")[speRefLabel] = p.general
			}

			final := map[string]map[string]any{}
			for name, spec := range tt.spes {
				final[name] = spe(t, name, spec)
			}

			for name, add := range p.adds {
				if final[name] == nil {
					final[name] = spe(t, name, "{}")
				}

				mergeSpec(final[name]["spec"].(map[string]any), add)
			}

			all := make([]map[string]any, 0, len(final))

			for _, s := range final {
				problems, err := pss.ValidateSPE(s)
				require.NoError(t, err)
				assert.Empty(t, problems)

				all = append(all, s)
			}

			leftover := tt.leftover
			if leftover == nil {
				leftover = []string{}
			}

			assert.Equal(t, leftover, fired(t, bound, all...))
			assert.Len(t, p.leftover, len(fired(t, bound, all...)))
		})
	}
}

// The rule attaches no fix when no SPE covers anything.
func TestPodSecurityStandardsFix_Unexceptable(t *testing.T) {
	withFix(t)

	obj := patchedDeployment(t, `{spec: {template: {spec: {securityContext: {runAsUser: null, runAsNonRoot: null}}}}}`)

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{storage.GetResourceIndex(obj): obj})

	errorList := errors.NewLintRuleErrorsList()
	NewPodSecurityStandardsRule(m, errorList).Check(t.Context())

	require.Len(t, errorList.GetErrors(), 1)
	assert.Empty(t, errorList.GetFixes())
}

// --fix writes the template next to the pod's one, keeps the finding with a note on
// binding, and never overwrites a file.
func TestPodSecurityStandardsFix(t *testing.T) {
	withFix(t)

	modulePath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(modulePath, "templates", "app"), 0o755))

	obj := patchedDeployment(t, privilegedApp)
	obj.AbsPath = filepath.Join(modulePath, "templates", "app", "deployment.yaml")

	run := func() pkg.LinterError {
		m := mocks.NewModuleMock(minimock.NewController(t))
		m.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{storage.GetResourceIndex(obj): obj})
		m.GetPathMock.Return(modulePath)

		errorList := errors.NewLintRuleErrorsList()
		NewPodSecurityStandardsRule(m, errorList).Check(t.Context())

		fixes := errorList.GetFixes()
		require.Len(t, fixes, 1)
		fixes[0]()

		errs := errorList.GetErrors()
		require.Len(t, errs, 1, "the pod is not bound yet")

		return errs[0]
	}

	e := run()

	var note errors.FixNote
	require.True(t, stderrors.As(e.FixError, &note), e.FixError)
	assert.Equal(t, `Generated SecurityPolicyException app in templates/app/security-policy-exception.yaml.
Bind the pod to SecurityPolicyException app: add the label "security.deckhouse.io/security-policy-exception: app" to spec.template.metadata.labels of Deployment/app in .
Replace every description: TODO with the reason the component needs the allowance.`, string(note))

	file := filepath.Join(modulePath, "templates", "app", "security-policy-exception.yaml")
	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, `{{- if .Values.global.enabledModules | has "admission-policy-engine" }}
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicyException
metadata:
  name: app
  namespace: d8-test
  labels:
    heritage: deckhouse
    module: {{ .Chart.Name }}
spec:
  securityContext:
    privileged:
      allowedValue: true
      metadata:
        description: TODO
{{- end }}
`, string(content))

	require.NoError(t, os.WriteFile(file, []byte("mine"), 0o644))

	e = run()
	require.True(t, stderrors.As(e.FixError, &note), e.FixError)
	assert.True(t, strings.HasPrefix(string(note), "templates/app/security-policy-exception.yaml exists, not overwritten"), string(note))

	content, err = os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "mine", string(content))
}

func withFix(t *testing.T) {
	t.Helper()

	flags.Fix = true

	t.Cleanup(func() { flags.Fix = false })
}

// An SPE named after the object rendered already, e.g. by an earlier --fix, with the
// pod not bound to it: nothing to add, the binding is still to do.
func TestPodSecurityStandardsFix_UnboundSPE(t *testing.T) {
	withFix(t)

	obj := patchedDeployment(t, privilegedApp)
	s := storage.StoreObject{Unstructured: unstructured.Unstructured{Object: spe(t, "app", `{securityContext: {privileged: {allowedValue: true}}}`)}}

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{storage.GetResourceIndex(obj): obj, storage.GetResourceIndex(s): s})
	m.GetPathMock.Return(t.TempDir())

	errorList := errors.NewLintRuleErrorsList()
	NewPodSecurityStandardsRule(m, errorList).Check(t.Context())

	fixes := errorList.GetFixes()
	require.Len(t, fixes, 1)
	fixes[0]()

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Equal(t, `Bind the pod to SecurityPolicyException app: add the label "security.deckhouse.io/security-policy-exception: app" to spec.template.metadata.labels of Deployment/app in .`,
		errs[0].FixError.Error())
}

// Violations no SPE covers are listed in the note apart from the generated SPE.
func TestPodSecurityStandardsFix_Leftover(t *testing.T) {
	withFix(t)

	modulePath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(modulePath, "templates"), 0o755))

	obj := patchedDeployment(t, `{spec: {template: {spec: {containers: [{name: app, securityContext: {privileged: true, capabilities: {drop: null}}}]}}}}`)
	obj.AbsPath = filepath.Join(modulePath, "templates", "deployment.yaml")

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{storage.GetResourceIndex(obj): obj})
	m.GetPathMock.Return(modulePath)

	errorList := errors.NewLintRuleErrorsList()
	NewPodSecurityStandardsRule(m, errorList).Check(t.Context())

	fixes := errorList.GetFixes()
	require.Len(t, fixes, 1)
	fixes[0]()

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Equal(t, `Generated SecurityPolicyException app in templates/security-policy-exception.yaml.
Bind the pod to SecurityPolicyException app: add the label "security.deckhouse.io/security-policy-exception: app" to spec.template.metadata.labels of Deployment/app in .
Replace every description: TODO with the reason the component needs the allowance.
No SecurityPolicyException covers the rest, fix the pod spec:
- D8AllowedCapabilities: container is not dropping all required capabilities, container: app | capabilities.drop: [] | policy allows: ["ALL"]
- D8AllowedCapabilities: no SecurityPolicyException can cover an empty capabilities.drop, add drop: [ALL] to containers (computed by dmt): app`,
		errs[0].FixError.Error())
}
