package fsutils

import (
	"os"
	"path/filepath"
	"regexp"
)

func GetFiles(rootPath string, skipSymlink bool, filters ...string) ([]string, error) {
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

		if filterPass(path, filters...) {
			result = append(result, path)
		}

		return nil
	})

	return result, err
}

func filterPass(path string, filters ...string) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		r, err := regexp.Compile(filter)
		if err != nil {
			continue
		}

		if r.MatchString(path) {
			return true
		}
	}

	return false
}
