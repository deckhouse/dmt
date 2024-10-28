package fsutils

import (
	"os"
	"path/filepath"
)

func GetFiles(rootPath string, skipSymlink bool) ([]string, error) {
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

		result = append(result, path)

		return nil
	})

	return result, err
}
