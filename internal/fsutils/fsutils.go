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

package fsutils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
)

// evalSymlinkCache is a cache for evaluated symlinks to avoid multiple evaluations
var evalSymlinkCache sync.Map

// MaxLintableFileSize bounds how large a file dmt will read into memory while
// linting. Files above it are generated data blobs (bundled Grafana dashboards,
// rendered openapi, CRD bundles), not hand-written sources; reading a
// multi-gigabyte file just to scan it would exhaust memory and flood the log.
const MaxLintableFileSize = 10 << 20 // 10 MiB

// ErrFileTooLarge is returned by ReadFile when a file exceeds MaxLintableFileSize.
var ErrFileTooLarge = errors.New("file too large to lint")

// ReadFile reads the named file like os.ReadFile but refuses files larger than
// MaxLintableFileSize, returning ErrFileTooLarge instead of loading a huge file
// into memory. Callers that scan discovered files should treat ErrFileTooLarge
// as "skip this file" rather than a hard failure.
func ReadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if info.Size() > MaxLintableFileSize {
		return nil, fmt.Errorf("%w: %s (%d bytes)", ErrFileTooLarge, path, info.Size())
	}

	return os.ReadFile(path)
}

// IsFileTooLarge reports whether err was produced by ReadFile refusing an
// oversized file. Linters that scan discovered files use it to skip such files
// rather than failing. It exists so callers need not import the standard errors
// package, which many of them alias to dmt's own errors package.
func IsFileTooLarge(err error) bool {
	return errors.Is(err, ErrFileTooLarge)
}

// IsDir checks if the given path is a directory
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// IsFile checks if the given path is a file
func IsFile(path string) bool {
	fi, err := os.Stat(path)

	return err == nil && !fi.IsDir()
}

// Getwd returns the current working directory.
func Getwd() (string, error) {
	var (
		cachedWd      string
		cachedWdError error
		getWdOnce     sync.Once
	)

	getWdOnce.Do(func() {
		cachedWd, cachedWdError = os.Getwd()
		if cachedWdError != nil {
			return
		}

		evaledWd, err := EvalSymlinks(cachedWd)
		if err != nil {
			cachedWd, cachedWdError = "", fmt.Errorf("can't eval symlinks on wd %s: %w", cachedWd, err)
			return
		}

		cachedWd = evaledWd
	})

	return cachedWd, cachedWdError
}

// EvalSymlinks returns the path name after the evaluation of any symbolic links.
func EvalSymlinks(path string) (string, error) {
	type evalSymlinkRes struct {
		path string
		err  error
	}

	r, ok := evalSymlinkCache.Load(path)
	if ok {
		er := r.(evalSymlinkRes)
		return er.path, er.err
	}

	var er evalSymlinkRes

	er.path, er.err = filepath.EvalSymlinks(path)
	evalSymlinkCache.Store(path, er)

	return er.path, er.err
}

// Rel returns a relative path from basepath to targpath.
func Rel(basepath, targpath string) string {
	rel, _ := filepath.Rel(basepath, targpath)
	return rel
}

// ExpandDir expands a path that starts with ~ to the user's home directory
// and returns the absolute path.
func ExpandDir(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	if path[0] != '~' {
		return filepath.Abs(path)
	}

	dir, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, path[1:]), nil
}

func FilterFileByExtensions(exts ...string) func(_, path string) bool {
	return func(_, path string) bool {
		return slices.Contains(exts, filepath.Ext(path))
	}
}

func FilterFileByNames(names ...string) func(_, path string) bool {
	return func(_, path string) bool {
		return slices.Contains(names, filepath.Base(path))
	}
}

func SplitManifests(data string) []string {
	// Split the data by "---" separator
	parts := regexp.MustCompile(`(?m)^---\s*$`).Split(data, -1)

	// Remove any leading or trailing whitespace from each part
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	// Filter out empty parts
	var nonEmptyParts []string

	for _, part := range parts {
		if part != "" {
			nonEmptyParts = append(nonEmptyParts, part)
		}
	}

	return nonEmptyParts
}
