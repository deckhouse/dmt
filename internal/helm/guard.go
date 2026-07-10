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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/werf/nelm/pkg/helm/pkg/ignore"
)

// symlinkLoop describes a symbolic link that resolves to one of its own
// ancestors and was therefore skipped while preparing a chart for rendering.
type symlinkLoop struct {
	// Path is the location of the offending link inside the chart.
	Path string
	// Resolved is the ancestor directory the link resolves to.
	Resolved string
}

// maxChartLoadBytes caps the cumulative size of the files nelm's chart loader
// would read into memory for a single chart.
//
// nelm's loader (GetFilesFromLocalFilesystem -> sympath.Walk) reads every
// regular file in the chart directory fully into RAM and follows symbolic links
// without any cycle detection. A symlink that resolves to one of its own
// ancestors, or one that points at a very large tree, therefore makes the walk
// read data without bound until the Go runtime aborts the process with an
// out-of-memory fatal error. This backstop is deliberately generous: a real
// chart's templates, values, CRDs and openapi schemas are far smaller, so
// tripping it means the directory is pathological rather than large.
const maxChartLoadBytes = 2 << 30 // 2 GiB

// loadChartIgnoreRules loads the chart's .helmignore rules exactly the way
// nelm's loader does (parse the file at the chart root if present, then add the
// built-in defaults), so scanning and cleaning consider the same set of files
// nelm would actually read.
func loadChartIgnoreRules(chartDir string) (*ignore.Rules, error) {
	rules := ignore.Empty()

	ifile := filepath.Join(chartDir, ignore.HelmIgnore)
	if _, err := os.Stat(ifile); err == nil {
		r, err := ignore.ParseFile(ifile)
		if err != nil {
			return nil, err
		}

		rules = r
	}

	rules.AddDefaults()

	return rules, nil
}

// isIgnored reports whether path (relative to the chart root) is excluded by the
// chart's .helmignore rules, matching how nelm's loader classifies entries.
func isIgnored(rules *ignore.Rules, root, path string, info os.FileInfo) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return rules.Ignore(filepath.ToSlash(rel), info)
}

// scanChartDir walks chartDir the same way nelm's loader does — following
// symbolic links and honouring .helmignore — but tracks the resolved real path
// of every directory currently on the descent stack so it can detect symlink
// loops, and sums the size of the regular files it would read so it can reject
// trees too large to load safely.
//
// It returns hasLoop=true when a symlink resolves to one of its own ancestors.
// The caller renders a cleaned copy of the directory in that case (see
// materializeCleanChartDir) instead of handing the loop to nelm's loader, which
// would follow it without bound and exhaust memory. An oversized tree is still a
// hard error, because cleaning cannot make it small enough to load safely.
//
// The walk only stats files (it never reads their contents), so this pass is
// cheap relative to the load nelm performs afterwards.
func scanChartDir(chartDir string) (bool, error) {
	rules, err := loadChartIgnoreRules(chartDir)
	if err != nil {
		return false, err
	}

	g := &chartDirGuard{root: chartDir, rules: rules, ancestors: make(map[string]struct{})}
	if err := g.walk(chartDir); err != nil {
		return false, err
	}

	return g.hasLoop, nil
}

type chartDirGuard struct {
	root      string
	rules     *ignore.Rules
	ancestors map[string]struct{}
	total     int64
	hasLoop   bool
}

func (g *chartDirGuard) walk(dir string) error {
	// Resolve to the real path so a symlink pointing back at an ancestor is
	// recognised as a cycle rather than followed forever.
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("evaluate symlink %s: %w", dir, err)
	}

	if _, ok := g.ancestors[real]; ok {
		g.hasLoop = true

		return nil
	}

	g.ancestors[real] = struct{}{}
	defer delete(g.ancestors, real)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := filepath.Join(dir, entry.Name())

		// os.Stat follows symlinks, matching how the loader classifies entries.
		info, err := os.Stat(name)
		if err != nil {
			return err
		}

		if isIgnored(g.rules, g.root, name, info) {
			continue
		}

		if info.IsDir() {
			if err := g.walk(name); err != nil {
				return err
			}

			continue
		}

		if !info.Mode().IsRegular() {
			continue
		}

		g.total += info.Size()
		if g.total > maxChartLoadBytes {
			return fmt.Errorf(
				"chart %q exceeds the %d-byte load limit; check for oversized files or a symbolic link pointing at a large tree",
				g.root, maxChartLoadBytes,
			)
		}
	}

	return nil
}

// materializeCleanChartDir copies chartDir into a fresh temporary directory,
// dereferencing symbolic links into real files and directories and skipping any
// directory that would form a symlink loop (a link resolving to one of its own
// ancestors). Files excluded by the chart's .helmignore are not copied, so the
// copy mirrors what nelm's loader would read. The returned directory therefore
// contains no symlinks and no cycles, so nelm's loader can walk it safely.
//
// It returns the path to the copy, the symlink loops it skipped (so the caller
// can report them), and a cleanup function the caller must invoke (typically via
// defer) to remove the copy.
func materializeCleanChartDir(chartDir string) (string, func(), []symlinkLoop, error) {
	rules, err := loadChartIgnoreRules(chartDir)
	if err != nil {
		return "", nil, nil, err
	}

	dst, err := os.MkdirTemp("", "dmt-chart-*")
	if err != nil {
		return "", nil, nil, err
	}

	cleanup := func() { _ = os.RemoveAll(dst) }

	c := &chartCleaner{root: chartDir, rules: rules, ancestors: make(map[string]struct{})}
	if err := c.copy(chartDir, dst); err != nil {
		cleanup()

		return "", nil, nil, err
	}

	return dst, cleanup, c.loops, nil
}

type chartCleaner struct {
	root      string
	rules     *ignore.Rules
	ancestors map[string]struct{}
	loops     []symlinkLoop
}

func (c *chartCleaner) copy(srcDir, dstDir string) error {
	real, err := filepath.EvalSymlinks(srcDir)
	if err != nil {
		return fmt.Errorf("evaluate symlink %s: %w", srcDir, err)
	}

	if _, ok := c.ancestors[real]; ok {
		c.loops = append(c.loops, symlinkLoop{Path: srcDir, Resolved: real})

		return nil
	}

	c.ancestors[real] = struct{}{}
	defer delete(c.ancestors, real)

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		// os.Stat follows symlinks so a symlinked file or directory is
		// materialized as its real target rather than copied as a link.
		info, err := os.Stat(srcPath)
		if err != nil {
			return err
		}

		if isIgnored(c.rules, c.root, srcPath, info) {
			continue
		}

		switch {
		case info.IsDir():
			if err := c.copy(srcPath, dstPath); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyRegularFile(srcPath, dstPath, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyRegularFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()

		return err
	}

	return out.Close()
}
