package fsutils

import (
	"os"
	"path/filepath"
)

type fFn func(string) bool

func GetFiles(rootPath string, skipSymlink bool, filters ...fFn) ([]string, error) {
	var result []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, _ error) error {
		if skipSymlink && info.Mode()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}

		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if filterPass(Rel(rootPath, path), filters...) {
			result = append(result, path)
		}

		return nil
	})

	return result, err
}

func filterPass(path string, filters ...fFn) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		if filter(path) {
			return true
		}
	}

	return false
}
