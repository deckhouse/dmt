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

package test

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/deckhouse/dmt/internal/moduleloader"
	"github.com/deckhouse/dmt/pkg/config"
	pkgerrors "github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/testers"
	"github.com/deckhouse/dmt/pkg/testers/conversions/tester"
)

type moduleResult struct {
	name    string
	failed  bool
	skipped bool
	err     error
}

type Manager struct {
	cfg     *config.RootConfig
	modules []string

	errors  *pkgerrors.TestErrorsList
	testers []testers.Tester
	results []moduleResult
}

func NewManager(dir string, rootConfig *config.RootConfig) (*Manager, error) {
	m := &Manager{
		cfg:    rootConfig,
		errors: pkgerrors.NewTestErrorsList(),
	}

	var err error
	m.modules, err = moduleloader.GetModulePaths(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get module paths: %w", err)
	}
	m.registerTesters()

	return m, nil
}

func (m *Manager) Run() {
	if len(m.modules) == 0 {
		fmt.Fprintf(os.Stderr, "⚠️ No modules found\n")
		return
	}

	for _, modulePath := range m.modules {
		result := m.runModuleTests(modulePath)
		m.results = append(m.results, result)
	}
}

func (m *Manager) runModuleTests(modulePath string) moduleResult {
	moduleName := extractModuleName(modulePath)
	failed, lastErr := m.runTesters(modulePath)

	if errors.Is(lastErr, testers.ErrNotApplicable) {
		return moduleResult{name: moduleName, skipped: true}
	}

	return moduleResult{name: moduleName, failed: failed, err: lastErr}
}

func (m *Manager) runTesters(modulePath string) (bool, error) {
	moduleName := extractModuleName(modulePath)
	var lastErr error
	anyTesterRan := false

	for _, tester := range m.testers {
		err := tester.Run(modulePath)
		if err != nil {
			if errors.Is(err, testers.ErrNotApplicable) {
				if lastErr == nil {
					lastErr = err
				}
				continue
			}
			anyTesterRan = true
			lastErr = err
			m.errors.WithTestID("test").
				WithModule(moduleName).
				Errorf("%s", err.Error())
		} else {
			anyTesterRan = true
		}
	}

	if !anyTesterRan {
		return false, lastErr
	}

	return lastErr != nil, lastErr
}

func (m *Manager) PrintResult() {
	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	boldRed := color.New(color.FgRed, color.Bold).SprintFunc()

	for _, result := range m.results {
		if result.skipped {
			continue
		}

		if result.failed {
			fmt.Fprintf(os.Stderr, "%s %s: %s\n", red("❌"), result.name, boldRed(result.err.Error()))
		} else {
			fmt.Printf("%s %s\n", green("✅"), result.name)
		}
	}
}

func (m *Manager) HasCriticalErrors() bool {
	return m.errors.ContainsErrors()
}

func (m *Manager) registerTesters() {
	m.testers = []testers.Tester{
		tester.New(),
	}
}

func extractModuleName(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		return path[idx+1:]
	}
	return path
}
