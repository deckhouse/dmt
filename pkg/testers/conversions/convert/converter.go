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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func TestConvert(rawSettings, rawExpected, pathToConversions string, fromVersion, toVersion int) error {
	converter, err := newConverter(pathToConversions)
	if err != nil {
		return err
	}

	settings, err := parseYAML(rawSettings)
	if err != nil {
		return err
	}

	converted, err := converter.ConvertTo(fromVersion, toVersion, settings)
	if err != nil {
		return err
	}

	expected, err := parseYAML(rawExpected)
	if err != nil {
		return err
	}

	if !mapsEqual(converted, expected) {
		return fmt.Errorf("expected: %v got: %v", expected, converted)
	}

	return nil
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

func mapsEqual(a, b map[string]any) bool {
	return string(must(json.Marshal(a))) == string(must(json.Marshal(b)))
}

func must[T any](v T, _ error) T {
	return v
}

func isConversionFile(name string) bool {
	return filepath.Ext(name) == ".yaml" && name != "testcases.yaml"
}
