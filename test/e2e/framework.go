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

// Package e2e provides an end-to-end testing framework for dmt.
//
// Cases are organized as testdata/<linter>/<case>/, grouping each case under
// the linter it primarily exercises. A case directory contains:
//   - expected.yaml: the case specification (description + expected findings).
//   - a module subdirectory (default "module/") that is linted as if the user
//     ran `dmt lint <module>`.
//
// A case is executed by copying its module into an isolated temp directory and
// running the full lint manager against it, then matching the structured
// findings against the expectations declared in expected.yaml. This exercises
// the real lint pipeline (config loading, helm render, every linter) rather
// than a single rule in isolation, while still letting each case assert on
// concrete, human-readable outcomes.
//
// A case may instead set "kind: conversions" to run the `dmt test conversions`
// testers; those results are adapted into the same finding shape (linter ID
// "conversions") so the same expectation matching applies.
package e2e

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/test"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
)

// Case kinds. A case either lints a module (KindLint, the default) or runs the
// `dmt test conversions` testers against it (KindConversions).
const (
	KindLint        = "lint"
	KindConversions = "conversions"
)

// Finding declares one expected lint finding for a case.
//
// Matching semantics:
//   - linter is required and matched case-insensitively against LinterID.
//   - rule, level and textContains are optional; when set they all must match.
//   - textContains is a case-sensitive substring match against the message.
//   - count is the expected number of matching findings; 0 means "at least one".
type Finding struct {
	Linter       string `yaml:"linter"`
	Rule         string `yaml:"rule"`
	Level        string `yaml:"level"`
	TextContains string `yaml:"textContains"`
	Count        int    `yaml:"count"`
}

func (f Finding) String() string {
	parts := []string{"linter=" + f.Linter}
	if f.Rule != "" {
		parts = append(parts, "rule="+f.Rule)
	}
	if f.Level != "" {
		parts = append(parts, "level="+f.Level)
	}
	if f.TextContains != "" {
		parts = append(parts, fmt.Sprintf("textContains=%q", f.TextContains))
	}

	return strings.Join(parts, " ")
}

// CaseSpec is the schema of an expected.yaml file.
type CaseSpec struct {
	// Description is a human-readable summary of what the case verifies.
	Description string `yaml:"description"`
	// Kind selects what to run against the module: "lint" (default) runs the
	// full lint pipeline, "conversions" runs the `dmt test conversions` testers.
	// For conversions cases, findings are exposed with linter ID "conversions"
	// and ObjectID set to the test name, so the same expectations apply.
	Kind string `yaml:"kind"`
	// Module is the subdirectory (relative to the case dir) that gets linted.
	// Defaults to "module".
	Module string `yaml:"module"`
	// ExpectClean asserts that the lint produced zero findings.
	ExpectClean bool `yaml:"expectClean"`
	// Expect lists the findings that must be present.
	Expect []Finding `yaml:"expect"`
	// Exhaustive, when true, asserts that there are no findings beyond those
	// listed in Expect (every produced finding must be matched by some Finding).
	Exhaustive bool `yaml:"exhaustive"`
}

// LoadCaseSpec reads and parses the expected.yaml file from a case directory.
func LoadCaseSpec(caseDir string) (*CaseSpec, error) {
	data, err := os.ReadFile(filepath.Join(caseDir, "expected.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read expected.yaml: %w", err)
	}

	spec := &CaseSpec{}
	if err := yaml.Unmarshal(data, spec); err != nil {
		return nil, fmt.Errorf("parse expected.yaml: %w", err)
	}

	if spec.Module == "" {
		spec.Module = "module"
	}

	if spec.Kind == "" {
		spec.Kind = KindLint
	}

	return spec, nil
}

// Run executes a case (lint or conversions) against a module directory and
// returns the produced findings.
func Run(kind, moduleDir string) ([]pkg.LinterError, error) {
	switch kind {
	case KindConversions:
		return RunConversions(moduleDir)
	case KindLint, "":
		return Lint(moduleDir)
	default:
		return nil, fmt.Errorf("unknown case kind %q", kind)
	}
}

// Lint runs the dmt lint pipeline against a module directory and returns all
// findings. The module is copied into an isolated temp directory first so the
// run is hermetic (no config inherited from parent dirs, no artifacts written
// back into testdata).
func Lint(moduleDir string) ([]pkg.LinterError, error) {
	tmpRoot, err := os.MkdirTemp("", "dmt-e2e-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpRoot)

	target := filepath.Join(tmpRoot, filepath.Base(moduleDir))
	if err := copyDir(moduleDir, target); err != nil {
		return nil, fmt.Errorf("copy module: %w", err)
	}

	// The lint manager relies on a couple of process-global knobs that are
	// normally set by cobra flags. Set sane defaults for the in-process run.
	if flags.LintersLimit <= 0 {
		flags.LintersLimit = 10
	}
	flags.LinterName = ""
	flags.ValuesFile = ""

	cfg, err := config.NewDefaultRootConfig(target)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Initialize the metrics client so linters that emit metrics don't panic.
	metrics.GetClient(target)

	mng := manager.NewManager(target, cfg)
	mng.Run()

	return mng.GetErrors(), nil
}

// RunConversions runs the `dmt test conversions` testers against a module
// directory and returns the results adapted to the common finding shape
// (LinterID "conversions", ObjectID set to the test name).
func RunConversions(moduleDir string) ([]pkg.LinterError, error) {
	tmpRoot, err := os.MkdirTemp("", "dmt-e2e-conv-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpRoot)

	target := filepath.Join(tmpRoot, filepath.Base(moduleDir))
	if err := copyDir(moduleDir, target); err != nil {
		return nil, fmt.Errorf("copy module: %w", err)
	}

	cfg, err := config.NewDefaultRootConfig(target)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	mng, err := test.NewManager(target, cfg)
	if err != nil {
		return nil, fmt.Errorf("create test manager: %w", err)
	}

	mng.Run()

	return adaptTestErrors(mng.GetErrors()), nil
}

func adaptTestErrors(errs []pkg.TestError) []pkg.LinterError {
	out := make([]pkg.LinterError, 0, len(errs))

	for i := range errs {
		out = append(out, pkg.LinterError{
			LinterID: "conversions",
			ModuleID: errs[i].ModuleID,
			ObjectID: errs[i].TestName,
			Text:     errs[i].Text,
			Level:    errs[i].Level,
		})
	}

	return out
}

// MatchResult describes the outcome of matching findings against expectations.
type MatchResult struct {
	// Failures contains human-readable descriptions of unmet expectations.
	Failures []string
}

// OK reports whether every expectation was satisfied.
func (r MatchResult) OK() bool { return len(r.Failures) == 0 }

// Match compares the produced findings against a case spec.
func Match(spec *CaseSpec, findings []pkg.LinterError) MatchResult {
	var res MatchResult

	if spec.ExpectClean && len(findings) > 0 {
		res.Failures = append(res.Failures,
			fmt.Sprintf("expected clean run but got %d finding(s):\n%s",
				len(findings), formatFindings(findings)))

		return res
	}

	matched := make([]bool, len(findings))

	for _, exp := range spec.Expect {
		var hits int

		for i := range findings {
			if findingMatches(exp, findings[i]) {
				hits++
				matched[i] = true
			}
		}

		switch {
		case exp.Count > 0 && hits != exp.Count:
			res.Failures = append(res.Failures,
				fmt.Sprintf("expected %d finding(s) for [%s], got %d", exp.Count, exp, hits))
		case exp.Count == 0 && hits == 0:
			res.Failures = append(res.Failures,
				fmt.Sprintf("expected at least one finding for [%s], got none", exp))
		}
	}

	if spec.Exhaustive {
		for i := range findings {
			if !matched[i] {
				res.Failures = append(res.Failures,
					fmt.Sprintf("unexpected finding (exhaustive mode): %s", formatFinding(findings[i])))
			}
		}
	}

	return res
}

func findingMatches(exp Finding, got pkg.LinterError) bool {
	if !strings.EqualFold(exp.Linter, got.LinterID) {
		return false
	}

	if exp.Rule != "" && exp.Rule != got.RuleID {
		return false
	}

	if exp.Level != "" && !strings.EqualFold(exp.Level, got.Level.String()) {
		return false
	}

	if exp.TextContains != "" && !strings.Contains(got.Text, exp.TextContains) {
		return false
	}

	return true
}

func formatFindings(findings []pkg.LinterError) string {
	var b strings.Builder

	for i := range findings {
		fmt.Fprintf(&b, "  - %s\n", formatFinding(findings[i]))
	}

	return b.String()
}

func formatFinding(f pkg.LinterError) string {
	return fmt.Sprintf("linter=%s rule=%s level=%s text=%q", f.LinterID, f.RuleID, f.Level, f.Text)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}
