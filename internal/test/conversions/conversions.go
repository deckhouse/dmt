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

package conversions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itchyny/gojq"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/test"
)

const (
	conversionsFolder = "openapi/conversions"
	testCasesFile     = "conversions_test.yaml"
)

// TestCase represents a single conversion test case
type TestCase struct {
	Name            string `yaml:"name" json:"name"`
	Settings        string `yaml:"settings" json:"settings"`
	Expected        string `yaml:"expected" json:"expected"`
	CurrentVersion  int    `yaml:"currentVersion" json:"currentVersion"`
	ExpectedVersion int    `yaml:"expectedVersion" json:"expectedVersion"`
}

// TestCasesFile represents the structure of the test cases YAML file
type TestCasesFile struct {
	Cases []TestCase `yaml:"cases" json:"cases"`
}

// Tester implements the test.Tester interface for conversion tests
type Tester struct{}

// NewTester creates a new conversions tester
func NewTester() *Tester {
	return &Tester{}
}

// Type returns the test type
func (*Tester) Type() test.TestType {
	return test.TestTypeConversions
}

// CanRun checks if conversion tests can be run for the given module path
func (*Tester) CanRun(modulePath string) bool {
	testFilePath := filepath.Join(modulePath, conversionsFolder, testCasesFile)
	_, err := os.Stat(testFilePath)
	return err == nil
}

// Run executes the conversion tests for the given module path
func (*Tester) Run(modulePath string) (*test.TestSuiteResult, error) {
	conversionsPath := filepath.Join(modulePath, conversionsFolder)
	testFilePath := filepath.Join(conversionsPath, testCasesFile)

	// Read test cases
	testCases, err := readTestCases(testFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cases: %w", err)
	}

	// Create converter
	converter, err := newConverter(conversionsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create converter: %w", err)
	}

	result := &test.TestSuiteResult{
		Type:    test.TestTypeConversions,
		Module:  filepath.Base(modulePath),
		Results: make([]test.TestResult, 0, len(testCases.Cases)),
	}

	for _, tc := range testCases.Cases {
		tr := test.TestResult{
			Name:   tc.Name,
			Passed: true,
		}

		err := runTestCase(converter, tc)
		if err != nil {
			tr.Passed = false
			tr.Message = err.Error()
		}

		result.Results = append(result.Results, tr)
	}

	return result, nil
}

func readTestCases(path string) (*TestCasesFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var testCases TestCasesFile
	if err := yaml.Unmarshal(data, &testCases); err != nil {
		return nil, err
	}

	return &testCases, nil
}

func runTestCase(converter *Converter, tc TestCase) error {
	settings, err := readSettings(tc.Settings)
	if err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	_, converted, err := converter.ConvertTo(tc.CurrentVersion, tc.ExpectedVersion, settings)
	if err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	expected, err := readSettings(tc.Expected)
	if err != nil {
		return fmt.Errorf("failed to parse expected: %w", err)
	}

	marshaledConverted, err := json.Marshal(converted)
	if err != nil {
		return fmt.Errorf("failed to marshal converted: %w", err)
	}

	marshaledExpected, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf("failed to marshal expected: %w", err)
	}

	if !bytes.Equal(marshaledConverted, marshaledExpected) {
		return fmt.Errorf("mismatch:\n  expected: %s\n  got:      %s", marshaledExpected, marshaledConverted)
	}

	return nil
}

func readSettings(settings string) (map[string]any, error) {
	var parsed map[string]any
	err := yaml.Unmarshal([]byte(settings), &parsed)
	return parsed, err
}

// Converter handles conversion between config versions
type Converter struct {
	latest      int
	conversions map[int]string
}

func newConverter(pathToConversions string) (*Converter, error) {
	c := &Converter{conversions: make(map[int]string), latest: 1}

	conversionsDir, err := os.ReadDir(pathToConversions)
	if err != nil {
		return nil, err
	}

	for _, file := range conversionsDir {
		if file.IsDir() {
			continue
		}

		ext := filepath.Ext(file.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		// Skip test files
		if file.Name() == testCasesFile {
			continue
		}

		v, conversion, err := readConversion(filepath.Join(pathToConversions, file.Name()))
		if err != nil {
			return nil, err
		}

		if v > c.latest {
			c.latest = v
		}
		c.conversions[v] = conversion
	}

	return c, nil
}

func readConversion(path string) (int, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}

	var parsed struct {
		Version     int      `yaml:"version"`
		Conversions []string `yaml:"conversions"`
	}

	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return 0, "", err
	}

	return parsed.Version, strings.Join(parsed.Conversions, " | "), nil
}

// ConvertTo converts settings from currentVersion to version
func (c *Converter) ConvertTo(currentVersion, version int, settings map[string]any) (int, map[string]any, error) {
	if currentVersion == c.latest || settings == nil || c.conversions == nil {
		return currentVersion, settings, nil
	}

	if version == 0 {
		version = c.latest
	}

	var err error
	for currentVersion++; currentVersion <= version; currentVersion++ {
		if settings, err = c.convert(currentVersion, settings); err != nil {
			return currentVersion, nil, err
		}
	}

	return c.latest, settings, err
}

func (c *Converter) convert(version int, settings map[string]any) (map[string]any, error) {
	conversion := c.conversions[version]
	if conversion == "" {
		return nil, errors.New("conversion not found")
	}

	query, err := gojq.Parse(conversion)
	if err != nil {
		return nil, err
	}

	v, _ := query.Run(settings).Next()
	if v == nil {
		return nil, nil
	}

	if err, ok := v.(error); ok {
		return nil, err
	}

	filtered, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("cannot unmarshal after converting")
	}

	return filtered, nil
}
