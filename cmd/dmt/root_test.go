package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunLintMultiple(t *testing.T) {
	// Test with empty directories list
	err := runLintMultiple([]string{})
	assert.NoError(t, err, "Should handle empty directories list")

	// Test with single directory
	err = runLintMultiple([]string{"."})
	assert.NoError(t, err, "Should handle single directory")

	// Test with multiple directories
	err = runLintMultiple([]string{".", "."})
	assert.NoError(t, err, "Should handle multiple directories")
}

func TestLintCmdFunc(t *testing.T) {
	// Test with no arguments (should default to current directory)
	// This is a basic test to ensure the function doesn't panic
	// In a real test, we would need to mock the dependencies
}
