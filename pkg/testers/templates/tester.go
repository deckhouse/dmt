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

// Package templates implements the "templates" tester. It renders a module's
// chart with user-supplied values and compares the result against committed
// golden snapshots, in the spirit of deckhouse's testing/helm harness.
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/module"
	pkgerrors "github.com/deckhouse/dmt/pkg/errors"
)

const (
	// ID is the tester identifier surfaced in results.
	ID = "templates"
	// TestsDirName is the per-module directory that holds template test cases.
	TestsDirName = "templates-tests"
	// valuesFileName is the per-case values file (optional).
	valuesFileName = "values.yaml"
	// snapshotFileName is the per-case golden rendered output.
	snapshotFileName = "expected.yaml"
)

type Tester struct {
	name, desc string
	update     bool

	ErrorList *pkgerrors.TestErrorsList
}

// New creates a templates tester. When update is true, mismatching snapshots are
// rewritten on disk instead of being reported as failures.
func New(errorList *pkgerrors.TestErrorsList, update bool) *Tester {
	return &Tester{
		name:      ID,
		desc:      "Renders module templates with provided values and compares them against golden snapshots",
		update:    update,
		ErrorList: errorList.WithTestGroup(ID),
	}
}

func (t *Tester) Name() string { return t.name }
func (t *Tester) Desc() string { return t.desc }

// Run executes the templates tester against the given module path.
// Returns true if the tester was applicable (i.e. the module ships test cases).
func (t *Tester) Run(modulePath string) bool {
	testsDir := filepath.Join(modulePath, TestsDirName)

	cases, err := discoverCases(testsDir)
	if err != nil {
		t.ErrorList.WithModule(filepath.Base(modulePath)).
			Errorf("%s", err.Error())

		return true
	}

	if len(cases) == 0 {
		return false
	}

	moduleName := filepath.Base(modulePath)
	errorList := t.ErrorList.WithModule(moduleName)

	for _, c := range cases {
		t.runCase(modulePath, c, errorList)
	}

	return true
}

type testCase struct {
	name         string
	dir          string
	valuesPath   string
	snapshotPath string
}

// discoverCases returns the test cases under testsDir. A case is any direct
// subdirectory; the values file is optional.
func discoverCases(testsDir string) ([]testCase, error) {
	entries, err := os.ReadDir(testsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read templates tests dir: %w", err)
	}

	var cases []testCase

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(testsDir, entry.Name())
		cases = append(cases, testCase{
			name:         entry.Name(),
			dir:          dir,
			valuesPath:   filepath.Join(dir, valuesFileName),
			snapshotPath: filepath.Join(dir, snapshotFileName),
		})
	}

	sort.Slice(cases, func(i, j int) bool { return cases[i].name < cases[j].name })

	return cases, nil
}

func (t *Tester) runCase(modulePath string, c testCase, errorList *pkgerrors.TestErrorsList) {
	errorList = errorList.WithTestName(c.name)

	userValues, err := loadValues(c.valuesPath)
	if err != nil {
		errorList.Errorf("testcase %q: %s", c.name, err.Error())
		return
	}

	files, err := module.RenderModuleWithValues(modulePath, userValues)
	if err != nil {
		errorList.Errorf("testcase %q: render failed: %s", c.name, err.Error())
		return
	}

	rendered, err := normalizeManifests(files)
	if err != nil {
		errorList.Errorf("testcase %q: %s", c.name, err.Error())
		return
	}

	if t.update {
		if err := os.WriteFile(c.snapshotPath, []byte(rendered), 0o644); err != nil {
			errorList.Errorf("testcase %q: write snapshot: %s", c.name, err.Error())
		}

		return
	}

	expected, err := os.ReadFile(c.snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			errorList.AddTestResult(
				fmt.Sprintf("testcase %q: missing snapshot %q (run with --update to create it)", c.name, snapshotFileName),
				rendered,
				"",
			)

			return
		}

		errorList.Errorf("testcase %q: read snapshot: %s", c.name, err.Error())

		return
	}

	if string(expected) != rendered {
		errorList.AddTestResult(
			fmt.Sprintf("testcase %q: rendered output does not match snapshot", c.name),
			rendered,
			string(expected),
		)
	}
}

// loadValues reads and parses the optional per-case values file.
func loadValues(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}

		return nil, fmt.Errorf("read values: %w", err)
	}

	values := map[string]any{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("parse values: %w", err)
	}

	return values, nil
}

// normalizeManifests renders the file->manifests map into a deterministic,
// canonical YAML stream: files sorted by path, each document re-marshalled with
// sorted keys and prefixed with its source path.
func normalizeManifests(files map[string]string) (string, error) {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	var sb strings.Builder

	for _, path := range paths {
		for _, doc := range splitYAMLDocuments(files[path]) {
			var obj any
			if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
				return "", fmt.Errorf("parse rendered manifest %q: %w", path, err)
			}

			if obj == nil {
				continue
			}

			canonical, err := yaml.Marshal(obj)
			if err != nil {
				return "", fmt.Errorf("marshal rendered manifest %q: %w", path, err)
			}

			sb.WriteString("---\n# Source: ")
			sb.WriteString(path)
			sb.WriteString("\n")
			sb.Write(canonical)
		}
	}

	return sb.String(), nil
}

// splitYAMLDocuments splits a multi-document YAML string on lines containing
// only a document separator.
func splitYAMLDocuments(content string) []string {
	var (
		docs    []string
		current strings.Builder
	)

	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "---" {
			docs = append(docs, current.String())
			current.Reset()

			continue
		}

		current.WriteString(line)
		current.WriteString("\n")
	}

	docs = append(docs, current.String())

	return docs
}
