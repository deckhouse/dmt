package container

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestApplyContainerRules_NoContainers(t *testing.T) {
	cfg := &pkg.ContainerLinterConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := &Container{cfg: cfg}

	obj := storage.StoreObject{
		AbsPath: "test.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "Pod",
				"metadata": map[string]any{"name": "test-obj"},
				"spec":     map[string]any{},
			},
		},
	}

	linter.applyContainerRules(obj, errList)
	errs := errList.GetErrors()
	assert.NotEmpty(t, errs, "Should report errors for missing labels and security context")
	var foundModule, foundHeritage, foundSecurityContext bool
	for _, e := range errs {
		if e.Text == "Object does not have the label \"module\"" {
			foundModule = true
		}
		if e.Text == "Object does not have the label \"heritage\"" {
			foundHeritage = true
		}
		if e.Text == "Object's SecurityContext is not defined" {
			foundSecurityContext = true
		}
	}
	assert.True(t, foundModule, "Should report missing 'module' label")
	assert.True(t, foundHeritage, "Should report missing 'heritage' label")
	assert.True(t, foundSecurityContext, "Should report missing SecurityContext")
}

func TestApplyContainerRules_ContainersError(t *testing.T) {
	cfg := &pkg.ContainerLinterConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := &Container{cfg: cfg}

	obj := storage.StoreObject{
		AbsPath: "test.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":     "UnknownKind",
				"metadata": map[string]any{"name": "test-obj"},
			},
		},
	}

	linter.applyContainerRules(obj, errList)
	assert.NotEmpty(t, errList.GetErrors(), "Error expected if GetAllContainers returns error")
}

func TestApplyContainerRules_AllRules(t *testing.T) {
	cfg := &pkg.ContainerLinterConfig{}
	errList := errors.NewLintRuleErrorsList()
	linter := &Container{cfg: cfg}

	obj := storage.StoreObject{
		AbsPath: "test.yaml",
		Unstructured: unstructured.Unstructured{
			Object: map[string]any{
				"kind":       "Deployment",
				"apiVersion": "apps/v1",
				"metadata": map[string]any{
					"name":      "test-deploy",
					"namespace": "test-ns",
					// no labels to trigger label rules
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "c1",
									"image": "nginx:latest",
									"env": []any{
										map[string]any{"name": "FOO", "value": "1"},
										map[string]any{"name": "FOO", "value": "2"}, // duplicate env
									},
									"ports": []any{
										map[string]any{"containerPort": 80},
									},
									// no securityContext to trigger rule
								},
								map[string]any{
									"name":  "c1", // duplicate name
									"image": "nginx@sha256:1234567890abcdef",
								},
							},
							// no liveness/readiness probes
						},
					},
					// no revisionHistoryLimit, priorityClassName, dnsPolicy
				},
			},
		},
	}

	linter.applyContainerRules(obj, errList)
	errs := errList.GetErrors()
	var (
		foundModule, foundHeritage, foundNameDup, foundEnvDup, foundSecCtx, foundLiveness, foundReadiness bool
	)
	for _, e := range errs {
		if e.Text == "Object does not have the label \"module\"" {
			foundModule = true
		}
		if e.Text == "Object does not have the label \"heritage\"" {
			foundHeritage = true
		}
		if e.Text == "Duplicate container name" {
			foundNameDup = true
		}
		if e.Text == "Container has two env variables with same name" {
			foundEnvDup = true
		}
		if e.Text == "Object's SecurityContext is not defined" {
			foundSecCtx = true
		}
		if e.Text == "Container does not contain liveness-probe" {
			foundLiveness = true
		}
		if e.Text == "Container does not contain readiness-probe" {
			foundReadiness = true
		}
	}
	assert.True(t, foundModule, "Should report missing 'module' label")
	assert.True(t, foundHeritage, "Should report missing 'heritage' label")
	assert.True(t, foundNameDup, "Should report duplicate container name")
	assert.True(t, foundEnvDup, "Should report duplicate env var")
	assert.True(t, foundSecCtx, "Should report missing SecurityContext")
	assert.True(t, foundLiveness, "Should report missing liveness probe")
	assert.True(t, foundReadiness, "Should report missing readiness probe")
}
