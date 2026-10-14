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

package pss

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func joinPaths(paths [][]string) []string {
	res := make([]string, 0, len(paths))
	for _, p := range paths {
		res = append(res, strings.Join(p, "."))
	}

	return res
}

// A change of this list means the CRD got (or lost) an allowance: make sure the
// description rule should cover it.
func TestSPEAllowancePaths(t *testing.T) {
	paths, err := SPEAllowancePaths()
	require.NoError(t, err)

	assert.Equal(t, []string{
		"network.hostIPC",
		"network.hostNetwork",
		"network.hostPID",
		"network.hostPorts.[]",
		"securityContext.allowPrivilegeEscalation",
		"securityContext.appArmorProfile",
		"securityContext.capabilities",
		"securityContext.privileged",
		"securityContext.procMount",
		"securityContext.readOnlyRootFilesystem",
		"securityContext.runAsNonRoot",
		"securityContext.runAsUser",
		"securityContext.seLinuxOptions",
		"securityContext.seccompProfile",
		"securityContext.sysctls",
		"volumes.hostPath.allowedValues.[]",
		"volumes.types",
	}, joinPaths(paths))
}

const fixtureCRD = `
spec:
  versions:
  - name: v1alpha1
    served: true
    schema:
      openAPIV3Schema:
        properties:
          spec:
            properties:
              network:
                properties:
                  hostNetwork:
                    properties:
                      allowedValue: {type: boolean}
                      metadata: {type: object, additionalProperties: true}
              newSection:
                properties:
                  newAllowance:
                    type: array
                    items:
                      properties:
                        name: {type: string}
                        metadata:
                          type: object
                          properties:
                            description: {type: string}
`

func TestAllowancePathsFixture(t *testing.T) {
	paths, err := allowancePaths([]byte(fixtureCRD))
	require.NoError(t, err)
	assert.Equal(t, []string{"network.hostNetwork", "newSection.newAllowance.[]"}, joinPaths(paths))
}

func TestAllowancePathsErrors(t *testing.T) {
	for name, doc := range map[string]string{
		"not yaml":          "spec: [",
		"no version":        "spec: {versions: [{name: v1, served: true}]}",
		"not served":        strings.Replace(fixtureCRD, "served: true", "served: false", 1),
		"no spec schema":    "spec: {versions: [{name: v1alpha1, served: true}]}",
		"map of allowances": strings.Replace(fixtureCRD, "              newSection:\n", "              newSection:\n                additionalProperties: {type: object}\n", 1),
		"no allowances":     "spec: {versions: [{name: v1alpha1, served: true, schema: {openAPIV3Schema: {properties: {spec: {properties: {a: {type: string}}}}}}}]}",
	} {
		_, err := allowancePaths([]byte(doc))
		assert.Error(t, err, name)
	}
}

func TestDefaultSPE(t *testing.T) {
	var obj map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(`
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicyException
metadata: {name: spe, namespace: d8-test}
spec:
  securityContext:
    capabilities: {allowedValues: {}}
  volumes:
    hostPath: {allowedValues: [{path: /var/log}, {path: /etc, readOnly: true}]}
`), &obj))

	orig, err := jsonCopy(obj)
	require.NoError(t, err)

	got, err := DefaultSPE(obj)
	require.NoError(t, err)

	assert.Equal(t, orig, obj, "input must not be modified")
	assert.Equal(t, map[string]any{
		"securityContext": map[string]any{
			"capabilities": map[string]any{"allowedValues": map[string]any{"drop": []any{"all"}, "add": []any{}}},
		},
		"volumes": map[string]any{
			"hostPath": map[string]any{"allowedValues": []any{
				map[string]any{"path": "/var/log", "readOnly": false},
				map[string]any{"path": "/etc", "readOnly": true},
			}},
		},
	}, got["spec"])
}

func TestValidateSPE(t *testing.T) {
	for name, tc := range map[string]struct {
		spec string
		want []string
	}{
		"valid": {spec: `
securityContext:
  sysctls: {allowedValues: [{name: net.ipv4.ip_forward, value: "1"}]}
  appArmorProfile: {allowedValues: [unconfined]}
volumes:
  hostPath: {allowedValues: [{path: /var/log}]}
network:
  hostPorts: [{port: 53, protocol: UDP}]`},
		"sysctls as strings": {
			spec: `{securityContext: {sysctls: {allowedValues: [net.ipv4.ip_forward=1]}}}`,
			want: []string{`spec.securityContext.sysctls.allowedValues[0]: Invalid value: "string": spec.securityContext.sysctls.allowedValues[0] in body must be of type object: "string"`},
		},
		"appArmor pattern": {
			spec: `{securityContext: {appArmorProfile: {allowedValues: [RuntimeDefault]}}}`,
			want: []string{`spec.securityContext.appArmorProfile.allowedValues[0]: Invalid value: "RuntimeDefault": spec.securityContext.appArmorProfile.allowedValues[0] in body should match '^(runtime\/default|unconfined|localhost\/.+)$'`},
		},
		"unknown field": {
			spec: `{network: {hostNetwork: {allowedValue: true, allowed: true}}}`,
			want: []string{`spec.network.hostNetwork.allowed: unknown field`},
		},
		"missing required": {
			spec: `{securityContext: {capabilities: {metadata: {description: x}}}}`,
			want: []string{`spec.securityContext.capabilities.allowedValues: Required value`},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var spec map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(tc.spec), &spec))

			got, err := ValidateSPE(map[string]any{
				"apiVersion": SPEAPIVersion,
				"kind":       SPEKind,
				"metadata":   map[string]any{"name": "spe", "namespace": "d8-test"},
				"spec":       spec,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
