/*
Copyright 2026 Flant JSC

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

// Package render implements the "render" command. It discovers every module
// under a directory and renders each module's templates into a 'rendered'
// directory at the module root, using values generated from the module's
// openapi schemas (the same way dmt generates values while linting).
package render

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/spec"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/moduleloader"
	"github.com/deckhouse/dmt/internal/values"
)

const (
	// RenderedDirName is the per-module directory that receives the rendered output.
	RenderedDirName = "rendered"
	// openAPIDirName is the per-module directory holding the values schemas.
	openAPIDirName = "openapi"
	// defaultValuesFile is the base openapi values schema file.
	defaultValuesFile = "values.yaml"
	// defaultEditionName is the directory name used for the base (non-edition)
	// render when edition-specific values files are present.
	defaultEditionName = "default"
	// editionValuesPrefix / editionValuesSuffix bound the edition name in an
	// edition-specific values schema file (e.g. "values_ce.yaml" -> "ce").
	editionValuesPrefix = "values_"
	editionValuesSuffix = ".yaml"
)

// Render discovers all modules under dir (including subdirectories) and renders
// each module's templates.
//
// When outputDir is empty, every module is rendered into a 'rendered' directory
// at its own root. When outputDir is set, all modules are rendered into a shared
// '<outputDir>/rendered/<module-name>/<edition>/' tree instead; outputDir must
// be an existing directory.
func Render(dir, outputDir string) error {
	expandedDir, err := fsutils.ExpandDir(dir)
	if err != nil {
		return fmt.Errorf("failed to expand directory: %w", err)
	}

	baseRenderedDir, err := prepareOutputDir(outputDir)
	if err != nil {
		return err
	}

	paths, err := moduleloader.GetModulePaths(expandedDir)
	if err != nil {
		return fmt.Errorf("failed to get module paths: %w", err)
	}

	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "⚠️ No modules found\n")
		return nil
	}

	globalValues, err := values.GetGlobalValues(getRootDirectory(expandedDir))
	if err != nil {
		return fmt.Errorf("failed to get global values: %w", err)
	}

	var hasErrors bool

	for _, modulePath := range paths {
		moduleName := moduleName(modulePath)

		log.Info("Rendering module", slog.String("module", moduleName), slog.String("path", modulePath))

		var renderErr error
		if baseRenderedDir != "" {
			renderErr = renderModuleToOutput(modulePath, globalValues, moduleName, baseRenderedDir)
		} else {
			renderErr = renderModule(modulePath, globalValues)
		}

		if renderErr != nil {
			log.Error("Failed to render module", slog.String("module", moduleName), log.Err(renderErr))

			hasErrors = true

			continue
		}
	}

	if hasErrors {
		return fmt.Errorf("failed to render some modules")
	}

	return nil
}

// prepareOutputDir validates the user-supplied output directory and returns the
// path of the 'rendered' directory to create inside it. It returns an empty
// string when outputDir is empty (per-module output mode).
func prepareOutputDir(outputDir string) (string, error) {
	if outputDir == "" {
		return "", nil
	}

	expandedOut, err := fsutils.ExpandDir(outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand output directory: %w", err)
	}

	info, err := os.Stat(expandedOut)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat output directory %q: %w", expandedOut, err)
		}
		// Directory does not exist — create it.
		if mkErr := os.MkdirAll(expandedOut, 0o755); mkErr != nil {
			return "", fmt.Errorf("create output directory %q: %w", expandedOut, mkErr)
		}
	} else if !info.IsDir() {
		return "", fmt.Errorf("output path %q is not a directory", expandedOut)
	}

	baseRenderedDir := filepath.Join(expandedOut, RenderedDirName)
	if err := os.MkdirAll(baseRenderedDir, 0o755); err != nil {
		return "", fmt.Errorf("create rendered directory: %w", err)
	}

	return baseRenderedDir, nil
}

// renderModule renders a single module into its 'rendered' directory, which is
// recreated on every run.
//
// When the module ships edition-specific values schemas
// ('openapi/values_<edition>.yaml'), the output is split per edition:
//
//	rendered/default        rendered from openapi/values.yaml
//	rendered/<edition>       rendered from openapi/values_<edition>.yaml
//
// Otherwise the manifests are written directly under 'rendered'.
func renderModule(modulePath string, globalSchema *spec.Schema) error {
	editions, err := discoverEditions(modulePath)
	if err != nil {
		return err
	}

	outputDir := filepath.Join(modulePath, RenderedDirName)

	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("clean rendered directory: %w", err)
	}

	if len(editions) == 0 {
		return renderEdition(modulePath, globalSchema, defaultValuesFile, outputDir)
	}

	if err := renderEdition(modulePath, globalSchema, defaultValuesFile, filepath.Join(outputDir, defaultEditionName)); err != nil {
		return fmt.Errorf("edition %q: %w", defaultEditionName, err)
	}

	for edition, valuesFile := range editions {
		if err := renderEdition(modulePath, globalSchema, valuesFile, filepath.Join(outputDir, edition)); err != nil {
			return fmt.Errorf("edition %q: %w", edition, err)
		}
	}

	return nil
}

// renderModuleToOutput renders a single module into the shared output tree at
// '<baseRenderedDir>/<moduleName>/<edition>/'. The 'default' edition (from
// openapi/values.yaml) is always rendered; any 'openapi/values_<edition>.yaml'
// files add further editions. The module's subtree is recreated on every run.
func renderModuleToOutput(modulePath string, globalSchema *spec.Schema, moduleName, baseRenderedDir string) error {
	editions, err := discoverEditions(modulePath)
	if err != nil {
		return err
	}

	moduleOut := filepath.Join(baseRenderedDir, moduleName)

	if err := os.RemoveAll(moduleOut); err != nil {
		return fmt.Errorf("clean module output directory: %w", err)
	}

	if err := renderEdition(modulePath, globalSchema, defaultValuesFile, filepath.Join(moduleOut, defaultEditionName)); err != nil {
		return fmt.Errorf("edition %q: %w", defaultEditionName, err)
	}

	for edition, valuesFile := range editions {
		if err := renderEdition(modulePath, globalSchema, valuesFile, filepath.Join(moduleOut, edition)); err != nil {
			return fmt.Errorf("edition %q: %w", edition, err)
		}
	}

	return nil
}

// renderEdition renders the module using the given openapi values schema file
// and writes the manifests into targetDir, preserving each manifest's
// chart-relative path (e.g. 'templates/foo.yaml').
func renderEdition(modulePath string, globalSchema *spec.Schema, valuesFile, targetDir string) error {
	files, err := module.RenderModuleForValuesFile(modulePath, globalSchema, valuesFile)
	if err != nil {
		return err
	}

	for path, content := range files {
		if strings.TrimSpace(content) == "" {
			continue
		}

		relPath := stripChartName(path)

		target := filepath.Join(targetDir, relPath)

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create directory for %q: %w", relPath, err)
		}

		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write rendered file %q: %w", relPath, err)
		}
	}

	return nil
}

// discoverEditions returns the edition-specific values schema files found in the
// module's openapi directory, keyed by edition name (e.g. "ce" ->
// "values_ce.yaml").
func discoverEditions(modulePath string) (map[string]string, error) {
	entries, err := os.ReadDir(filepath.Join(modulePath, openAPIDirName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read openapi directory: %w", err)
	}

	editions := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, editionValuesPrefix) || !strings.HasSuffix(name, editionValuesSuffix) {
			continue
		}

		edition := strings.TrimSuffix(strings.TrimPrefix(name, editionValuesPrefix), editionValuesSuffix)
		if edition == "" {
			continue
		}

		editions[edition] = name
	}

	return editions, nil
}

// moduleName returns the module's name taken from its 'module.yaml' (falling
// back to 'Chart.yaml' and finally to the directory name).
func moduleName(modulePath string) string {
	moduleYaml, err := module.ParseModuleConfigFile(modulePath)
	if err != nil {
		moduleYaml = nil
	}

	chartYaml, err := module.ParseChartFile(modulePath)
	if err != nil {
		chartYaml = nil
	}

	if name := module.GetModuleName(moduleYaml, chartYaml); name != "" {
		return name
	}

	return filepath.Base(modulePath)
}

// stripChartName drops the leading chart-name component from a rendered manifest
// path (e.g. "module/templates/foo.yaml" -> "templates/foo.yaml").
func stripChartName(path string) string {
	elements := strings.Split(path, "/")
	if len(elements) > 1 {
		return filepath.Join(elements[1:]...)
	}

	return path
}

// getRootDirectory walks up from dir looking for a deckhouse repository root
// (one that ships global-hooks/openapi values). When found, those global values
// are used; otherwise the embedded defaults are used.
func getRootDirectory(dir string) string {
	for {
		if fsutils.IsDir(filepath.Join(dir, "global-hooks", "openapi")) &&
			fsutils.IsDir(filepath.Join(dir, "modules")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "config-values.yaml")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "values.yaml")) {
			return dir
		}

		parent := filepath.Dir(dir)
		if dir == parent || parent == "" {
			break
		}

		dir = parent
	}

	return ""
}
