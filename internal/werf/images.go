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

package werf

import (
	"cmp"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/fsutils"
)

const (
	imagesDirName   = "images"
	werfIncFileName = "werf.inc.yaml"
)

// ModuleImageWerfFile holds a rendered werf.inc.yaml file together with its
// path relative to the module directory (e.g. "images/<name>/werf.inc.yaml").
type ModuleImageWerfFile struct {
	// RelPath is the module-relative path of the source werf.inc.yaml file.
	RelPath string
	// Content is the rendered werf.inc.yaml content.
	Content string
}

// GetModuleImagesWerfFiles renders every "images/*/werf.inc.yaml" file found in
// the module directory and returns them keyed by their module-relative path.
//
// Unlike GetWerfConfig, it does not walk up to the repository root werf.yaml and
// does not aggregate stages/base images from outside the module. Only the
// module's own images are considered. Files that cannot be rendered in isolation
// (e.g. they rely on build context that is only available to the full werf
// pipeline) are skipped rather than reported as errors.
func GetModuleImagesWerfFiles(moduleDir string) ([]ModuleImageWerfFile, error) {
	imagesDir := filepath.Join(moduleDir, imagesDirName)
	if !fsutils.IsDir(imagesDir) {
		return nil, nil
	}

	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		return nil, err
	}

	var result []ModuleImageWerfFile

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		incPath := filepath.Join(imagesDir, entry.Name(), werfIncFileName)
		if !fsutils.IsFile(incPath) {
			continue
		}

		content, err := os.ReadFile(incPath)
		if err != nil {
			return nil, err
		}

		rendered, err := renderWerfIncFile(moduleDir, entry.Name(), string(content))
		if err != nil {
			// Best-effort: the inc file may rely on build context that is only
			// available to the full werf pipeline. Skip it instead of producing
			// a false positive.
			log.Debug("skipping werf.inc.yaml that cannot be rendered in isolation",
				slog.String("path", incPath),
				slog.String("error", err.Error()),
			)

			continue
		}

		result = append(result, ModuleImageWerfFile{
			RelPath: filepath.ToSlash(filepath.Join(imagesDirName, entry.Name(), werfIncFileName)),
			Content: rendered,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].RelPath < result[j].RelPath })

	return result, nil
}

// renderWerfIncFile renders a single images/<name>/werf.inc.yaml file the same
// way the module's ".werf/images.yaml" would: the content is treated as a
// template and executed with a per-image context.
func renderWerfIncFile(moduleDir, imageName, content string) (string, error) {
	tmpl := template.New(werfIncFileName)
	tmpl.Funcs(funcMap(tmpl))

	if err := parseWerfConfigTemplatesDir(moduleDir, tmpl); err != nil {
		return "", err
	}

	tmpl, err := tmpl.Parse(content)
	if err != nil {
		return "", err
	}

	root := map[string]any{
		"Files": NewFiles(filepath.Join(moduleDir, werfFileName), moduleDir),
		"Env":   cmp.Or(os.Getenv("WERF_ENV"), "EE"),
	}

	ctx := map[string]any{
		"Root":             root,
		"ImageName":        imageName,
		"ImagePath":        filepath.ToSlash(filepath.Join("/", imagesDirName, imageName)),
		"ModuleNamePrefix": "",
		"ModuleDir":        "/",
		"GOPROXY":          cmp.Or(os.Getenv("GOPROXY"), "https://proxy.golang.org,direct"),
	}

	return executeTemplate(tmpl, werfIncFileName, ctx)
}
