package openapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBilingualCRDFiles(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"crds/foo.yaml",
		"crds/doc-ru-orphan.yaml",
		"crds/foo-tests.yaml",
		"crds/sub/nested.yaml",
		"openapi/config-values.yaml",
	}

	for _, file := range files {
		fullPath := filepath.Join(dir, file)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte("type: object"), 0o600))
	}

	got := bilingualCRDFiles(dir)
	gotRel := make([]string, 0, len(got))
	for _, file := range got {
		rel, err := filepath.Rel(dir, file)
		require.NoError(t, err)
		gotRel = append(gotRel, rel)
	}

	require.ElementsMatch(t, []string{
		filepath.Join("crds", "foo.yaml"),
		filepath.Join("crds", "doc-ru-orphan.yaml"),
	}, gotRel)
}

func TestBilingualCRDFilesMissingCRDsDir(t *testing.T) {
	require.Empty(t, bilingualCRDFiles(t.TempDir()))
}
