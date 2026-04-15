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

	pkgerrors "github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/testers/conversions/convert"
)

const (
	ID                = "conversions"
	conversionsFolder = "openapi/conversions"
	configValuesFile  = "openapi/config-values.yaml"
)

type testcase struct {
	Name            string `json:"name"`
	Settings        string `json:"settings"`
	Expected        string `json:"expected"`
	CurrentVersion  int    `json:"currentVersion"`
	ExpectedVersion int    `json:"expectedVersion"`
}

type testcasesFile struct {
	Testcases []testcase `json:"testcases"`
}

type Tester struct {
	name, desc string
	ErrorList  *pkgerrors.TestErrorsList
}

func New(errorList *pkgerrors.TestErrorsList) *Tester {
	return &Tester{
		name:      ID,
		desc:      "Tests module conversion specifications against OpenAPI configs",
		ErrorList: errorList.WithTestID(ID),
	}
}

func (t *Tester) Name() string { return t.name }
func (t *Tester) Desc() string { return t.desc }

// Run executes the conversions tester against the given module path.
// Returns true if the tester was applicable to this module.
func (t *Tester) Run(modulePath string) bool {
	moduleName := filepath.Base(modulePath)
	errorList := t.ErrorList.WithModule(moduleName)

	convFolder, configVersion, applicable := t.checkConversions(modulePath, errorList)
	if !applicable {
		return false
	}

	latestVersion, err := t.parseConversions(convFolder)
	if err != nil {
		errorList.Errorf("%s", err.Error())
		return true
	}

	if latestVersion > 0 && configVersion != latestVersion {
		errorList.Errorf("x-config-version mismatch: expected latest conversion version %d, got x-config-version %d", latestVersion, configVersion)
		return true
	}

	return t.runConversions(convFolder, errorList)
}

// checkConversions verifies that the module has a conversions folder and a valid config version.
// Returns the convFolder path, configVersion, and whether the tester is applicable.
func (t *Tester) checkConversions(modulePath string, errorList *pkgerrors.TestErrorsList) (string, int, bool) {
	convFolder := filepath.Join(modulePath, conversionsFolder)

	hasConversions, err := hasConversionsFolder(convFolder)
	if err != nil {
		errorList.Error(err.Error())
		return "", 0, true
	}

	if !hasConversions {
		return "", 0, false
	}

	configVersion, err := getConfigVersion(modulePath)
	if err != nil {
		errorList.Errorf("%s", err.Error())
		return "", 0, true
	}

	if configVersion == 0 {
		return "", 0, false
	}

	return convFolder, configVersion, true
}

// parseConversions validates all conversion files and returns the latest version found.
// Returns an error if any conversion file is invalid (e.g., missing conversions array).
func (t *Tester) parseConversions(convFolder string) (int, error) {
	return convert.ValidateConversions(convFolder)
}

// runConversions executes testcases against the converter and records results into ErrorList.
// Returns true if testcases were found (tester is applicable), false if no testcases file exists.
func (t *Tester) runConversions(convFolder string, errorList *pkgerrors.TestErrorsList) bool {
	testcasesPath := filepath.Join(convFolder, "testcases.yaml")

	_, err := os.Stat(testcasesPath)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		errorList.Errorf("cannot stat testcases file: %s", err.Error())
		return true
	}

	testcases, err := parseTestcases(testcasesPath)
	if err != nil {
		errorList.Errorf("%s", err.Error())
		return true
	}

	for _, tc := range testcases {
		result, err := convert.TestConvert(tc.Name, tc.Settings, tc.Expected, convFolder, tc.CurrentVersion, tc.ExpectedVersion)
		if err != nil {
			errorList.Errorf("%s", err.Error())
			continue
		}

		if !result.Passed {
			errorList.AddTestResult(
				fmt.Sprintf("testcase %q: conversion mismatch", result.Name),
				result.Name,
				result.Got,
				result.Expected,
			)
		}
	}

	return true
}

func hasConversionsFolder(convFolder string) (bool, error) {
	stat, err := os.Stat(convFolder)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("cannot stat conversions folder: %w", err)
	}
	return stat.IsDir(), nil
}

func parseTestcases(path string) ([]testcase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read testcases file: %w", err)
	}

	var tc testcasesFile
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("cannot decode testcases file: %w", err)
	}

	return tc.Testcases, nil
}

func getConfigVersion(modulePath string) (int, error) {
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
