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

package convert

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/itchyny/gojq"
	"sigs.k8s.io/yaml"
)

type conversionFile struct {
	Version     int
	Conversions []string
}

type Converter struct {
	latestVersion int
	conversions   map[int]string
}

// ConvertResult holds the structured outcome of a test conversion.
// Error is nil when the conversion infrastructure works correctly.
// Passed indicates whether the conversion output matches the expected output.
type ConvertResult struct {
	Passed   bool
	Name     string
	Got      string // YAML of actual conversion result
	Expected string // YAML of expected conversion result
}

func newConverter(path string) (*Converter, error) {
	c := &Converter{
		conversions:   make(map[int]string),
		latestVersion: 1,
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read conversions dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isConversionFile(entry.Name()) {
			continue
		}

		conv, err := parseConversionFile(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, err
		}

		if conv.Version > c.latestVersion {
			c.latestVersion = conv.Version
		}
		c.conversions[conv.Version] = strings.Join(conv.Conversions, " | ")
	}

	return c, nil
}

func (c *Converter) ConvertTo(currentVersion, targetVersion int, settings map[string]any) (map[string]any, error) {
	if currentVersion >= c.latestVersion || settings == nil {
		return settings, nil
	}

	for currentVersion++; currentVersion <= targetVersion; currentVersion++ {
		result, err := c.applyConversion(currentVersion, settings)
		if err != nil {
			return nil, err
		}
		settings = result
	}

	return settings, nil
}

func (c *Converter) applyConversion(version int, settings map[string]any) (map[string]any, error) {
	rule, ok := c.conversions[version]
	if !ok {
		return nil, fmt.Errorf("conversion for version %d not found", version)
	}

	query, err := gojq.Parse(rule)
	if err != nil {
		return nil, fmt.Errorf("parse jq query: %w", err)
	}

	iter := query.Run(settings)
	result, ok := iter.Next()
	if !ok {
		return nil, nil
	}

	if err, ok := result.(error); ok {
		return nil, err
	}

	filtered, ok := result.(map[string]any)
	if !ok {
		return nil, errors.New("conversion result is not a map")
	}

	return filtered, nil
}

func TestConvert(name, rawSettings, rawExpected, pathToConversions string, fromVersion, toVersion int) (*ConvertResult, error) {
	converter, err := newConverter(pathToConversions)
	if err != nil {
		return nil, err
	}

	settings, err := parseYAML(rawSettings)
	if err != nil {
		return nil, err
	}

	converted, err := converter.ConvertTo(fromVersion, toVersion, settings)
	if err != nil {
		return nil, err
	}

	expected, err := parseYAML(rawExpected)
	if err != nil {
		return nil, err
	}

	if !mapsEqual(converted, expected) {
		return &ConvertResult{
			Passed:   false,
			Name:     name,
			Got:      formatYAML(converted),
			Expected: formatYAML(expected),
		}, nil
	}

	return &ConvertResult{
		Passed: true,
		Name:   name,
	}, nil
}

func formatYAML(data map[string]any) string {
	if data == nil {
		return "{}\n"
	}
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Sprintf("<error formatting yaml: %v>\n", err)
	}
	return string(out)
}

func mapsEqual(a, b map[string]any) bool {
	return cmp.Equal(a, b)
}

// ValidateConversions validates all conversion files in the given directory
// and returns the latest version found. Returns an error if any file is invalid
// (e.g., missing or empty conversions array, invalid version).
func ValidateConversions(convFolder string) (int, error) {
	entries, err := os.ReadDir(convFolder)
	if err != nil {
		return 0, fmt.Errorf("read conversions dir: %w", err)
	}

	latest := 0
	for _, entry := range entries {
		if entry.IsDir() || !isConversionFile(entry.Name()) {
			continue
		}

		conv, err := parseConversionFile(filepath.Join(convFolder, entry.Name()))
		if err != nil {
			return 0, err
		}

		if conv.Version > latest {
			latest = conv.Version
		}
	}

	return latest, nil
}

func parseConversionFile(path string) (conversionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return conversionFile{}, fmt.Errorf("read file: %w", err)
	}

	var conv conversionFile
	if err := yaml.Unmarshal(data, &conv); err != nil {
		return conversionFile{}, fmt.Errorf("unmarshal: %w", err)
	}

	if conv.Version < 1 {
		return conversionFile{}, fmt.Errorf("invalid conversion version %d in %s: must be >= 1", conv.Version, path)
	}

	if len(conv.Conversions) == 0 {
		return conversionFile{}, fmt.Errorf("empty conversions array in %s", path)
	}

	return conv, nil
}

func parseYAML(data string) (map[string]any, error) {
	var result map[string]any
	if err := yaml.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return result, nil
}

func isConversionFile(name string) bool {
	return filepath.Ext(name) == ".yaml" && name != "testcases.yaml"
}
