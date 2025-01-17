package werf

import (
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar"
)

type files struct {
	rootDir string
}

func NewFiles(rootDir string) files {
	return files{rootDir}
}

func (f files) Get(relPath string) string {
	var res []byte
	res, err := os.ReadFile(filepath.Join(f.rootDir, relPath))
	if err != nil {
		panic(err.Error())
	}

	return string(res)
}

func (f files) doGlob(pattern string) (map[string]any, error) {
	res := map[string]any{}
	matches, err := doublestar.Glob(filepath.Join(f.rootDir, pattern))
	if err != nil {
		return nil, err
	}
	for _, path := range matches {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		rel, _ := filepath.Rel(f.rootDir, path)
		res[rel] = string(data)
	}

	return res, nil
}

func (f files) Glob(pattern string) map[string]any {
	if res, err := f.doGlob(pattern); err != nil {
		panic(err.Error())
	} else {
		return res
	}
}
