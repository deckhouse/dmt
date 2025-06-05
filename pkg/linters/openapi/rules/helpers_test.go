package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func createTempFile(t *testing.T, content string) (string, func()) {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.yaml")

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return filePath, cleanup
}
