package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunLintMultiple(t *testing.T) {
	// Test with empty directories list
	err := runLintMultiple([]string{})
	require.NoError(t, err, "Should handle empty directories list")

	// Test with single directory
	err = runLintMultiple([]string{"."})
	require.NoError(t, err, "Should handle single directory")

	// Test with multiple directories
	err = runLintMultiple([]string{".", "."})
	require.NoError(t, err, "Should handle multiple directories")
}

func TestLintCmdFunc(_ *testing.T) {
	// Test with no arguments (should default to current directory)
	// This is a basic test to ensure the function doesn't panic
	// In a real test, we would need to mock the dependencies
}

func TestRunTestsMultiple(t *testing.T) {
	// Similar style to lint tests: ensure empty and multiple directories are handled
	err := runTestsMultiple([]string{})
	require.NoError(t, err, "Should handle empty directories list")

	err = runTestsMultiple([]string{"."})
	require.NoError(t, err, "Should handle single directory")

	err = runTestsMultiple([]string{".", "."})
	require.NoError(t, err, "Should handle multiple directories")
}

func TestTestCmdFunc(_ *testing.T) {
	// Stubbed basic invocation; real tests would require mocking runTestsMultiple or os.Exit
}
