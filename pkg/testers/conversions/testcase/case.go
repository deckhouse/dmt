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

package testcase

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg/testers"
	"github.com/deckhouse/dmt/pkg/testers/conversions/convert"
)

const conversionsFolder = "openapi/conversions"

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

func Run(modulePath string) error {
	testcasesPath := filepath.Join(modulePath, conversionsFolder, "testcases.yaml")

	_, err := os.Stat(testcasesPath)
	if os.IsNotExist(err) {
		return testers.NotApplicable("testcases.yaml is missing")
	}
	if err != nil {
		return fmt.Errorf("cannot stat testcases file: %w", err)
	}

	tc, err := parseTestcases(testcasesPath)
	if err != nil {
		return err
	}

	for _, c := range tc {
		err := convert.TestConvert(c.Name, c.Settings, c.Expected, filepath.Join(modulePath, conversionsFolder), c.CurrentVersion, c.ExpectedVersion)
		if err != nil {
			return err
		}
	}

	return nil
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
