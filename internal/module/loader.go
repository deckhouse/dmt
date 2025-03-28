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

package module

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/ignore"
)

// kind of fork from helmv3: pkg/chart/loader/directory.go
// but with Chart.yaml injection

var utf8bom = []byte{0xEF, 0xBB, 0xBF}

// LoadModuleAsChart loads a module as a chart
// default helm loader couldn't be used without Chart.yaml, but deckhouse module
// could exist without this file, deckhouse will create it automatically
func LoadModuleAsChart(moduleName, dir string) (*chart.Chart, error) {
	topdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	// Just used for errors.
	c := &chart.Chart{}

	rules := ignore.Empty()
	ifile := filepath.Join(topdir, ignore.HelmIgnore)
	if _, err = os.Stat(ifile); err == nil {
		r, err := ignore.ParseFile(ifile) //nolint:govet // copypaste from helmv3
		if err != nil {
			return c, err
		}
		rules = r
	}
	rules.AddDefaults()

	files := make([]*loader.BufferedFile, 0)
	topdir += string(filepath.Separator)

	chartFileExists := false

	walk := func(name string, fi os.FileInfo, err error) error {
		n := strings.TrimPrefix(name, topdir)
		if n == "" {
			// No need to process top level. Avoid bug with helmignore .* matching
			// empty names. See issue 1779.
			return nil
		}

		if n == "Chart.yaml" {
			chartFileExists = true
		}

		// Normalize to / since it will also work on Windows
		n = filepath.ToSlash(n)

		if err != nil {
			return err
		}
		if fi.IsDir() {
			// Directory-based ignore rules should involve skipping the entire
			// contents of that directory.
			if rules.Ignore(n, fi) {
				return filepath.SkipDir
			}
			return nil
		}

		// If a .helmignore file matches, skip this file.
		if rules.Ignore(n, fi) {
			return nil
		}

		// Irregular files include devices, sockets, and other uses of files that
		// are not regular files. In Go they have a file mode type bit set.
		// See https://golang.org/pkg/os/#FileMode for examples.
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("cannot load irregular file %s as it has file mode type bits set", name)
		}

		data, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", n, err)
		}

		data = bytes.TrimPrefix(data, utf8bom)

		files = append(files, &loader.BufferedFile{Name: n, Data: data})
		return nil
	}

	//nolint:gocritic // copypaste from helmv3
	if err = Walk(topdir, walk); err != nil {
		return c, err
	}

	if !chartFileExists {
		files = append(files, &loader.BufferedFile{
			Name: "Chart.yaml",
			Data: []byte(fmt.Sprintf("name: %s\nversion: 0.2.0\n", moduleName)),
		})
	}

	return loader.LoadFiles(files)
}
