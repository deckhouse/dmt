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

	"github.com/deckhouse/dmt/internal/moduleloader"
	"github.com/deckhouse/dmt/pkg/config"
	pkgerrors "github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/testers"
	"github.com/deckhouse/dmt/pkg/testers/conversions/tester"
)

type Manager struct {
	cfg     *config.RootConfig
	modules []string

	errors  *pkgerrors.TestErrorsList
	testers []testers.Tester
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
		m.runModuleTests(modulePath)
	}
}

func (m *Manager) runModuleTests(modulePath string) {
	moduleName := extractModuleName(modulePath)
	failed, lastErr := m.runTesters(modulePath)

	if errors.Is(lastErr, testers.ErrNotApplicable) {
		fmt.Fprintf(os.Stderr, "⚠️ %s: %s\n", moduleName, lastErr.Error())
		return
	}

	if failed {
		fmt.Fprintf(os.Stderr, "❌ %s: %s\n", moduleName, lastErr.Error())
	} else {
		fmt.Printf("✅ %s\n", moduleName)
	}
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
	// Errors are printed immediately during Run()
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
