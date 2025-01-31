package fsutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
