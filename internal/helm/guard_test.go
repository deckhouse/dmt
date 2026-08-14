/*
Copyright 2025 Flant JSC

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

package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestScanChartDirDetectsLoop verifies a symlink resolving to one of its own
// ancestors is reported as a loop, while a clean tree is not.
func TestScanChartDirDetectsLoop(t *testing.T) {
	clean := t.TempDir()
	writeGuardFile(t, filepath.Join(clean, "Chart.yaml"), "name: x\nversion: 0.1.0\n")
	writeGuardFile(t, filepath.Join(clean, "templates", "cm.yaml"), "kind: ConfigMap\n")

	hasLoop, err := scanChartDir(clean)
	require.NoError(t, err)
	require.False(t, hasLoop, "clean chart must not report a loop")

	looped := t.TempDir()
	writeGuardFile(t, filepath.Join(looped, "Chart.yaml"), "name: x\nversion: 0.1.0\n")
	require.NoError(t, os.Symlink(looped, filepath.Join(looped, "loop")))

	hasLoop, err = scanChartDir(looped)
	require.NoError(t, err)
	require.True(t, hasLoop, "symlink to an ancestor must be reported as a loop")
}

// TestMaterializeCleanChartDir verifies the cleaned copy drops the symlink loop
// and honours .helmignore, so nelm's loader receives a safe, loop-free tree that
// mirrors what it would actually read.
func TestMaterializeCleanChartDir(t *testing.T) {
	src := t.TempDir()

	writeGuardFile(t, filepath.Join(src, ".helmignore"), "*.ignored\n")
	writeGuardFile(t, filepath.Join(src, "Chart.yaml"), "name: x\nversion: 0.1.0\n")
	writeGuardFile(t, filepath.Join(src, "templates", "cm.yaml"), "kind: ConfigMap\n")
	writeGuardFile(t, filepath.Join(src, "big.ignored"), "excluded by .helmignore\n")

	// A symlink loop: loop -> the chart root (an ancestor of itself).
	require.NoError(t, os.Symlink(src, filepath.Join(src, "templates", "loop")))

	dst, cleanup, loops, err := materializeCleanChartDir(src)
	require.NoError(t, err)

	defer cleanup()

	// The skipped loop is reported back so the caller can surface it.
	require.Len(t, loops, 1, "the symlink loop must be reported")
	require.Equal(t, filepath.Join(src, "templates", "loop"), loops[0].Path)

	// Real chart content is materialized.
	require.FileExists(t, filepath.Join(dst, "Chart.yaml"))
	require.FileExists(t, filepath.Join(dst, "templates", "cm.yaml"))

	// .helmignore'd file is not copied.
	_, err = os.Stat(filepath.Join(dst, "big.ignored"))
	require.True(t, os.IsNotExist(err), "file excluded by .helmignore must not be copied")

	// The loop is dropped: the copy contains no symlinks and no re-entrant dir.
	_, err = os.Lstat(filepath.Join(dst, "templates", "loop"))
	require.True(t, os.IsNotExist(err), "symlink loop must not be present in the cleaned copy")

	require.NoError(t, filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		require.Zero(t, info.Mode()&os.ModeSymlink, "cleaned copy must not contain symlinks: %s", path)

		return nil
	}))
}

func writeGuardFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
