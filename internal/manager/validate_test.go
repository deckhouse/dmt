package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestValidateModule(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "Chart.yaml"), []byte("name: test-chart\nversion: 1.0.0"), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, "module.yaml"), []byte("name: test-chart\nnamespace: test-namespace"), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, ".namespace"), []byte("test-namespace"), 0600)
	_ = os.Mkdir(filepath.Join(tempDir, "openapi"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "openapi", "values.yaml"), []byte(""), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, "openapi", "config-values.yaml"), []byte(""), 0600)

	m := &Manager{
		errors: &errors.LintRuleErrorsList{},
	}

	err := m.validateModule(tempDir)
	require.NoError(t, err)
}

func TestGetNamespace(t *testing.T) {
	tempDir := t.TempDir()

	namespaceFile := filepath.Join(tempDir, ".namespace")
	_ = os.WriteFile(namespaceFile, []byte("test-namespace"), 0600)
	namespace := getNamespace(tempDir)
	assert.Equal(t, "test-namespace", namespace)

	_ = os.Remove(namespaceFile)
	namespace = getNamespace(tempDir)
	assert.Empty(t, namespace)
}

func TestValidateOpenAPIDir(t *testing.T) {
	tempDir := t.TempDir()

	err := validateOpenAPIDir(tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAPI dir does not exist")

	openAPIDir := filepath.Join(tempDir, "openapi")
	_ = os.Mkdir(openAPIDir, 0755)
	err = validateOpenAPIDir(tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAPI dir does not contain values.yaml")
	assert.Contains(t, err.Error(), "OpenAPI dir does not contain config-values.yaml")

	_ = os.WriteFile(filepath.Join(openAPIDir, "values.yaml"), []byte(""), 0600)
	_ = os.WriteFile(filepath.Join(openAPIDir, "config-values.yaml"), []byte(""), 0600)
	err = validateOpenAPIDir(tempDir)
	require.NoError(t, err)
}
