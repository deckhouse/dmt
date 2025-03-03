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
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mitchellh/go-homedir"
)

// IsDir checks if the given path is a directory
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// IsFile checks if the given path is a file
func IsFile(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}

// ShortestRelPath returns the shortest relative path from the working directory to the given path.
func ShortestRelPath(path, wd string) (string, error) {
	if wd == "" { // get it if user don't have cached working dir
		var err error
		wd, err = Getwd()
		if err != nil {
			return "", fmt.Errorf("can't get working directory: %w", err)
		}
	}

	evaledPath, err := EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("can't eval symlinks for path %s: %w", path, err)
	}
	path = evaledPath

	// make path absolute and then relative to be able to fix this case:
	// we are in /test dir, we want to normalize ../test, and have file file.go in this dir;
	// it must have normalized path file.go, not ../test/file.go,
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(wd, path)
	}

	relPath, err := filepath.Rel(wd, absPath)
	if err != nil {
		return "", fmt.Errorf("can't get relative path for path %s and root %s: %w",
			absPath, wd, err)
	}

	return relPath, nil
}

var (
	cachedWd      string
	cachedWdError error
	getWdOnce     sync.Once
)

// Getwd returns the current working directory.
func Getwd() (string, error) {
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

var evalSymlinkCache sync.Map

type evalSymlinkRes struct {
	path string
	err  error
}

// EvalSymlinks returns the path name after the evaluation of any symbolic links.
func EvalSymlinks(path string) (string, error) {
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

func IsFileExist(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func FilterFileByExtensions(exts ...string) func(_, path string) bool {
	return func(_, path string) bool {
		for _, ext := range exts {
			if filepath.Ext(path) == ext {
				return true
			}
		}

		return false
	}
}
