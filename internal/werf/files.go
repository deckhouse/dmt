package werf

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
)

type files struct {
	rootDir   string
	moduleDir string
}

func NewFiles(rootDir, moduleDir string) files {
	moduleDir, _ = filepath.Abs(moduleDir)
	return files{
		rootDir:   filepath.Dir(rootDir),
		moduleDir: moduleDir,
	}
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
	dir := f.rootDir
	// Check if we are looking for werf.inc.yaml in the module directory
	// If so, we need to change the directory to the module directory
	// and remove the modules/* prefix from the pattern
	// This is needed because the module directory is not a direct child of the root directory
	// and the pattern should be relative to the root directory
	// Specific for Deckhouse project
	if strings.Contains(pattern, "werf.inc.yaml") {
		dir = f.moduleDir
		pattern = strings.TrimPrefix(pattern, "modules/*")
	}
	matches, err := doublestar.Glob(filepath.Join(dir, pattern))
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
