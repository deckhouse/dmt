package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestDeckhouseCRDsRule(t *testing.T) {
	tests := []struct {
		name       string
		moduleName string
		content    string
		wantErrors []string
	}{
		{
			name:       "valid CRD",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: nil,
		},
		{
			name:       "invalid API version",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"`},
		},
		{
			name:       "missing heritage label",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD should contain "heritage = deckhouse" label`},
		},
		{
			name:       "wrong heritage label",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: wrong
    module: test-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD should contain "heritage = deckhouse" label, but got "heritage = wrong"`},
		},
		{
			name:       "missing module label",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD should contain "module = test-module" label`},
		},
		{
			name:       "wrong module label",
			moduleName: "test-module",
			content: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.deckhouse.io
  labels:
    heritage: deckhouse
    module: wrong-module
spec:
  group: deckhouse.io
  names:
    kind: Test
    plural: tests
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object`,
			wantErrors: []string{`CRD should contain "module = test-module" label, but got "module = wrong-module"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, cleanup := createTempFile(t, tt.content)
			defer cleanup()

			rule := NewDeckhouseCRDsRule(&config.OpenAPISettings{}, "test")
			errorList := errors.NewLintRuleErrorsList()
			rule.Run(tt.moduleName, filePath, errorList)

			errs := errorList.GetErrors()
			if tt.wantErrors == nil {
				assert.Empty(t, errs)
			} else {
				assert.Len(t, errs, len(tt.wantErrors))
				for i, err := range errs {
					assert.Contains(t, err.Text, tt.wantErrors[i])
				}
			}
		})
	}
}
