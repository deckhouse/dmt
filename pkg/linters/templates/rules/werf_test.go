package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestValidateWerfTemplates(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-image")
	if err := os.WriteFile(filePath, []byte(`image: {{ include "helm_lib_module_image" . "mock-module/test-image" }}`), 0644); err != nil {
		t.Fatal(err)
	}

	rule := NewWerfRule()
	errorList := errors.NewLintRuleErrorsList()

	// Mock module with valid Werf file
	mc := minimock.NewController(t)

	mock := mocks.NewModuleMock(mc)
	mock.GetPathMock.Return("/mock/path")
	mock.GetNameMock.Return("mock-module")
	mock.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{
		storage.ResourceIndex{
			Kind:      "Deployment",
			Name:      "test-deployment",
			Namespace: "test-namespace",
		}: {
			AbsPath: filePath,
		},
	})
	mock.GetWerfFileMock.Return(`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`)

	rule.ValidateWerfTemplates(mock, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid Werf file")

	errorList = errors.NewLintRuleErrorsList()
	// Mock module with invalid Werf file
	mockModuleWerfInvalid := mocks.NewModuleMock(mc)
	mockModuleWerfInvalid.GetPathMock.Return("/mock/path")
	mockModuleWerfInvalid.GetNameMock.Return("mock-module")
	mockModuleWerfInvalid.GetWerfFileMock.Return(`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
# Missing stageDependencies

`)
	mockModuleWerfInvalid.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{
		storage.ResourceIndex{
			Kind:      "Deployment",
			Name:      "test-deployment",
			Namespace: "test-namespace",
		}: {
			AbsPath: filePath,
		},
	})

	rule.ValidateWerfTemplates(mockModuleWerfInvalid, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid Werf file")
	assert.Contains(t, errorList.GetErrors()[0].Text, "'git.stageDependencies' is required")
}

func TestCheckGitSection(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	// Valid manifest
	validManifests := []string{
		`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`,
	}

	checkGitSection("mock-module", validManifests, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid manifest")

	// Invalid manifest
	invalidManifests := []string{
		`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  # Missing stageDependencies
`,
	}

	checkGitSection("mock-module", invalidManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid manifest")
	assert.Contains(t, errorList.GetErrors()[0].Text, "'git.stageDependencies' is required")

	// Malformed YAML
	malformedManifests := []string{
		`
image: mock-module/test-image
git:
  - stageDependencies: [build: "file1", "file2"]
`,
	}

	checkGitSection("mock-module", malformedManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for malformed YAML")
	assert.Contains(t, errorList.GetErrors()[0].Text, "parsing Werf file, document 1 (image: mock-module/test-image) failed")
}

func TestCheckTemplatesUsingRenderedImages(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-image")
	if err := os.WriteFile(filePath, []byte(`image: {{ include "helm_lib_module_image" . "mock-module/test-image" }}`), 0644); err != nil {
		t.Fatal(err)
	}

	errorList := errors.NewLintRuleErrorsList()

	// Mock module with valid Werf file
	mc := minimock.NewController(t)

	mock := mocks.NewModuleMock(mc)
	mock.GetStorageMock.Return(map[storage.ResourceIndex]storage.StoreObject{
		storage.ResourceIndex{
			Kind:      "Deployment",
			Name:      "test-deployment",
			Namespace: "test-namespace",
		}: {
			AbsPath: filePath,
			Unstructured: unstructured.Unstructured{
				Object: map[string]any{
					"image": "mock-module/test-image",
				},
			},
		},
	})
	mock.GetWerfFileMock.Return(`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`)

	for _, object := range mock.GetStorage() {
		checkTemplatesUsingRenderedImages(object, []string{mock.GetWerfFile()}, errorList)
	}
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid manifest")

	// Invalid manifest
	invalidManifests := []string{
		`
image: mock-module/test-image-invalid
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`,
	}

	for _, object := range mock.GetStorage() {
		checkTemplatesUsingRenderedImages(object, invalidManifests, errorList)
	}
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid manifest")
	assert.Contains(t, errorList.GetErrors()[0].Text, "image mock-module/test-image is not found in the manifests")
}

func TestCheckUnderscoredImages(t *testing.T) {
	errorList := errors.NewLintRuleErrorsList()

	// Valid manifest
	validManifests := []string{
		`
image: mock-module/test-image
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`,
	}

	checkUnderscoredImages(validManifests, errorList)
	assert.False(t, errorList.ContainsErrors(), "Expected no errors for valid manifest")

	// Invalid manifest
	invalidManifests := []string{
		`
image: mock-module/test-image_invalid
git:
- add: /deckhouse/modules/910-test-module/images/test-image
  to: /src
  stageDependencies:
    install:
    - '**/*.sh'
`,
	}

	checkUnderscoredImages(invalidManifests, errorList)
	assert.True(t, errorList.ContainsErrors(), "Expected errors for invalid manifest")
	assert.Contains(t, errorList.GetErrors()[0].Text, "image name should not contain underscores")
}
