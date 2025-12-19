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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTester is a mock implementation of Tester for testing
type mockTester struct {
	testType TestType
	canRun   bool
	results  *TestSuiteResult
	runErr   error
}

func (m *mockTester) Type() TestType {
	return m.testType
}

func (m *mockTester) CanRun(_ string) bool {
	return m.canRun
}

func (m *mockTester) Run(_ string) (*TestSuiteResult, error) {
	return m.results, m.runErr
}

func TestRunner_Register(t *testing.T) {
	runner := NewRunner()
	assert.Empty(t, runner.testers)

	runner.Register(&mockTester{testType: TestTypeConversions})
	assert.Len(t, runner.testers, 1)
}

func TestRunner_Run(t *testing.T) {
	tmpDir := t.TempDir()

	runner := NewRunner()
	runner.Register(&mockTester{
		testType: TestTypeConversions,
		canRun:   true,
		results: &TestSuiteResult{
			Type:   TestTypeConversions,
			Module: "test-module",
			Results: []TestResult{
				{Name: "test1", Passed: true},
				{Name: "test2", Passed: false, Message: "failed"},
			},
		},
	})

	summary, err := runner.Run(RunOptions{
		ModulePath: tmpDir,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, summary.TotalSuites)
	assert.Equal(t, 0, summary.PassedSuites)
	assert.Equal(t, 1, summary.FailedSuites)
	assert.Equal(t, 2, summary.TotalTests)
	assert.Equal(t, 1, summary.PassedTests)
	assert.Equal(t, 1, summary.FailedTests)
}

func TestRunner_Run_FilterByType(t *testing.T) {
	tmpDir := t.TempDir()

	runner := NewRunner()
	runner.Register(&mockTester{
		testType: TestTypeConversions,
		canRun:   true,
		results: &TestSuiteResult{
			Type:   TestTypeConversions,
			Module: "test-module",
			Results: []TestResult{
				{Name: "test1", Passed: true},
			},
		},
	})
	runner.Register(&mockTester{
		testType: "other-type",
		canRun:   true,
		results: &TestSuiteResult{
			Type:   "other-type",
			Module: "test-module",
			Results: []TestResult{
				{Name: "other-test", Passed: true},
			},
		},
	})

	// Filter to only conversions
	summary, err := runner.Run(RunOptions{
		ModulePath: tmpDir,
		TestTypes:  []TestType{TestTypeConversions},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, summary.TotalSuites)
	assert.Equal(t, 1, summary.TotalTests)
}

func TestRunner_Run_SkipsCannotRun(t *testing.T) {
	tmpDir := t.TempDir()

	runner := NewRunner()
	runner.Register(&mockTester{
		testType: TestTypeConversions,
		canRun:   false, // This tester cannot run
	})

	summary, err := runner.Run(RunOptions{
		ModulePath: tmpDir,
	})

	require.NoError(t, err)
	assert.Equal(t, 0, summary.TotalSuites)
	assert.Equal(t, 0, summary.TotalTests)
}
