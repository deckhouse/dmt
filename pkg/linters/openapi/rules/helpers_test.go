package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gojuno/minimock/v3"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
)

// moduleAt returns a module that reports path as its root. The rules only need
// GetPath here because the tests drive checkFile directly, bypassing the walk.
func moduleAt(t *testing.T, path string) pkg.Module {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(path)

	return m
}

func createTempFile(t *testing.T, content string) (string, func()) {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.yaml")

	err := os.WriteFile(filePath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return filePath, cleanup
}
