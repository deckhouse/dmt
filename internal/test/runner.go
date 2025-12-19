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

package test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
)

// TestType represents a type of test that can be run
type TestType string

const (
	TestTypeConversions TestType = "conversions"
)

// TestResult represents the result of a single test case
type TestResult struct {
	Name    string
	Passed  bool
	Message string
}

// TestSuiteResult represents the result of a test suite
type TestSuiteResult struct {
	Type    TestType
	Module  string
	Results []TestResult
}

// Tester is the interface that all test implementations must implement
type Tester interface {
	// Type returns the test type
	Type() TestType
	// CanRun checks if this tester can run for the given module path
	CanRun(modulePath string) bool
	// Run executes the tests for the given module path
	Run(modulePath string) (*TestSuiteResult, error)
}

// Runner manages and executes tests
type Runner struct {
	testers []Tester
}

// NewRunner creates a new test runner
func NewRunner() *Runner {
	return &Runner{
		testers: make([]Tester, 0),
	}
}

// Register adds a tester to the runner
func (r *Runner) Register(t Tester) {
	r.testers = append(r.testers, t)
}

// RunOptions contains options for running tests
type RunOptions struct {
	ModulePath string
	TestTypes  []TestType // empty means all
	Verbose    bool
}

// Summary contains the overall test results
type Summary struct {
	TotalSuites  int
	PassedSuites int
	FailedSuites int
	TotalTests   int
	PassedTests  int
	FailedTests  int
	Results      []*TestSuiteResult
}

// AddResult adds a test suite result and updates counters
func (s *Summary) AddResult(result *TestSuiteResult) {
	s.Results = append(s.Results, result)
	s.TotalSuites++

	s.TotalTests += result.Total()
	s.PassedTests += result.PassedCount()
	s.FailedTests += result.FailedCount()

	if result.IsPassed() {
		s.PassedSuites++
	} else {
		s.FailedSuites++
	}
}

// Total returns the total number of tests in the suite
func (r *TestSuiteResult) Total() int {
	return len(r.Results)
}

// PassedCount returns the number of passed tests
func (r *TestSuiteResult) PassedCount() int {
	count := 0
	for _, tr := range r.Results {
		if tr.Passed {
			count++
		}
	}
	return count
}

// FailedCount returns the number of failed tests
func (r *TestSuiteResult) FailedCount() int {
	return r.Total() - r.PassedCount()
}

// IsPassed returns true if all tests in the suite passed
func (r *TestSuiteResult) IsPassed() bool {
	return r.FailedCount() == 0
}

// Run executes tests based on the provided options
func (r *Runner) Run(opts RunOptions) (*Summary, error) {
	modulePath, err := filepath.Abs(opts.ModulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	if _, err := os.Stat(modulePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("module path does not exist: %s", modulePath)
	}

	summary := &Summary{
		Results: make([]*TestSuiteResult, 0),
	}

	for _, tester := range r.testers {
		if !matchesTestType(opts.TestTypes, tester.Type()) {
			continue
		}

		if !tester.CanRun(modulePath) {
			continue
		}

		result, err := tester.Run(modulePath)
		if err != nil {
			return nil, fmt.Errorf("test %s failed: %w", tester.Type(), err)
		}

		if result != nil {
			summary.AddResult(result)
		}
	}

	return summary, nil
}

// matchesTestType returns true if types is empty (run all) or contains the given type
func matchesTestType(types []TestType, t TestType) bool {
	if len(types) == 0 {
		return true
	}
	for _, tt := range types {
		if tt == t {
			return true
		}
	}
	return false
}

// PrintSummary prints the test summary to stdout
func PrintSummary(summary *Summary) {
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	bold := color.New(color.Bold)

	fmt.Println()
	bold.Println("Test Results:")
	fmt.Println("─────────────────────────────────────────")

	for _, suite := range summary.Results {
		fmt.Printf("\n📦 Module: %s\n", suite.Module)
		fmt.Printf("   Test Type: %s\n", suite.Type)

		for _, result := range suite.Results {
			if result.Passed {
				green.Printf("   ✓ %s\n", result.Name)
			} else {
				red.Printf("   ✗ %s\n", result.Name)
				if result.Message != "" {
					red.Printf("     → %s\n", result.Message)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("─────────────────────────────────────────")
	bold.Printf("Summary: ")

	if summary.FailedTests == 0 {
		green.Printf("%d passed", summary.PassedTests)
	} else {
		green.Printf("%d passed", summary.PassedTests)
		fmt.Print(", ")
		red.Printf("%d failed", summary.FailedTests)
	}

	fmt.Printf(" (%d total)\n", summary.TotalTests)

	if summary.TotalSuites == 0 {
		yellow.Println("No tests found")
	}
}
