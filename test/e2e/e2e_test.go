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

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestE2E discovers e2e cases under testdata/ and runs them, grouped by linter.
//
// The layout is testdata/<linter>/<case>/, where <linter> is the name of the
// linter the case primarily exercises and <case> is a directory that contains:
//   - expected.yaml describing the expected findings (see CaseSpec / Finding),
//   - a module/ subdirectory containing the Deckhouse module to lint.
//
// Each linter folder becomes a parent subtest and every case inside it a nested
// subtest, e.g. TestE2E/templates/vpa-pdb-absent.
func TestE2E(t *testing.T) {
	const root = "testdata"

	groups, err := os.ReadDir(root)
	require.NoError(t, err, "read testdata dir")

	var ranAny bool

	for _, group := range groups {
		if !group.IsDir() {
			continue
		}

		groupDir := filepath.Join(root, group.Name())

		cases := discoverCases(t, groupDir)
		if len(cases) == 0 {
			continue
		}

		ranAny = true

		t.Run(group.Name(), func(t *testing.T) {
			t.Parallel()

			for _, caseDir := range cases {
				name, err := filepath.Rel(groupDir, caseDir)
				require.NoError(t, err)

				t.Run(name, func(t *testing.T) {
					t.Parallel()
					runCase(t, caseDir)
				})
			}
		})
	}

	require.True(t, ranAny, "no e2e cases found under testdata/")
}

func runCase(t *testing.T, caseDir string) {
	t.Helper()

	spec, err := LoadCaseSpec(caseDir)
	require.NoError(t, err, "load case spec")

	if spec.Description != "" {
		t.Logf("case: %s", spec.Description)
	}

	moduleDir := filepath.Join(caseDir, spec.Module)
	require.DirExists(t, moduleDir, "module dir %q must exist", moduleDir)

	findings, err := Run(spec.Kind, moduleDir)
	require.NoError(t, err, "run case")

	result := Match(spec, findings)
	if !result.OK() {
		t.Fatalf("expectations not met for case %q:\n  %s\n\nall findings:\n%s",
			caseDir, joinFailures(result.Failures), formatFindings(findings))
	}
}

// discoverCases returns every case directory (one that contains an
// expected.yaml file) found anywhere beneath root.
func discoverCases(t *testing.T, root string) []string {
	t.Helper()

	var cases []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Name() == "expected.yaml" {
			cases = append(cases, filepath.Dir(path))
		}

		return nil
	})
	require.NoError(t, err, "walk %q", root)

	return cases
}

func joinFailures(failures []string) string {
	out := ""

	for i, f := range failures {
		if i > 0 {
			out += "\n  "
		}

		out += f
	}

	return out
}
