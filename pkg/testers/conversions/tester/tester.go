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

package tester

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg/testers"
	"github.com/deckhouse/dmt/pkg/testers/conversions/testcase"
)

const (
	ID                = "conversions"
	conversionsFolder = "openapi/conversions"
	configValuesFile  = "openapi/config-values.yaml"
)

type Tester struct {
	name, desc string
}

func New() *Tester {
	return &Tester{
		name: ID,
		desc: "Tests module conversion specifications against OpenAPI configs",
	}
}

func (t *Tester) Name() string { return t.name }
func (t *Tester) Desc() string { return t.desc }

func (t *Tester) Run(modulePath string) error {
	configVersion, err := t.getConfigVersion(modulePath)
	if err != nil {
		return err
	}

	if configVersion == 0 {
		return testers.NotApplicable("x-config-version is 0 or not set")
	}

	convFolder := filepath.Join(modulePath, conversionsFolder)
	if err := t.validateConversionsFolder(convFolder); err != nil {
		return err
	}

	latestVersion, err := t.getLatestConversionVersion(convFolder)
	if err != nil {
		return err
	}

	if latestVersion > 0 && configVersion != latestVersion {
		return fmt.Errorf(`
x-config-version mismatch: config-values.yaml has x-config-version %d, but latest conversion version is %d`, configVersion, latestVersion)
	}

	return testcase.Run(modulePath)
}

func (t *Tester) getLatestConversionVersion(convFolder string) (int, error) {
	entries, err := os.ReadDir(convFolder)
	if err != nil {
		return 0, fmt.Errorf("read conversions dir: %w", err)
	}

	latest := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" || entry.Name() == "testcases.yaml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(convFolder, entry.Name()))
		if err != nil {
			return 0, fmt.Errorf("read conversion file %s: %w", entry.Name(), err)
		}

		var conv struct {
			Version int `json:"version"`
		}
		if err := yaml.Unmarshal(data, &conv); err != nil {
			return 0, fmt.Errorf("unmarshal conversion file %s: %w", entry.Name(), err)
		}

		if conv.Version > latest {
			latest = conv.Version
		}
	}

	return latest, nil
}

func (t *Tester) getConfigVersion(modulePath string) (int, error) {
	configFilePath := filepath.Join(modulePath, configValuesFile)
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("cannot read config-values.yaml: %w", err)
	}

	var configValues struct {
		ConfigVersion int `json:"x-config-version"`
	}
	if err := yaml.Unmarshal(data, &configValues); err != nil {
		return 0, fmt.Errorf("cannot decode config-values.yaml: %w", err)
	}

	return configValues.ConfigVersion, nil
}

func (t *Tester) validateConversionsFolder(convFolder string) error {
	stat, err := os.Stat(convFolder)
	if err != nil || !stat.IsDir() {
		return fmt.Errorf("conversions folder is not exist or not a directory")
	}
	return nil
}
