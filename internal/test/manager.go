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
	"bytes"
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/go-wordwrap"

	"github.com/deckhouse/dmt/internal/moduleloader"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	pkgerrors "github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/testers"
	tester "github.com/deckhouse/dmt/pkg/testers/conversions"
)

type moduleResult struct {
	name    string
	tester  string
	failed  bool
	skipped bool
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
	failed, testerName := m.runTesters(modulePath, moduleName)

	if !failed && testerName == "" {
		return moduleResult{name: moduleName, skipped: true}
	}

	return moduleResult{name: moduleName, tester: testerName, failed: failed}
}

func (m *Manager) runTesters(modulePath, moduleName string) (bool, string) {
	var lastTesterName string
	var anyApplicable bool

	for _, t := range m.testers {
		lastTesterName = t.Name()
		applicable := t.Run(modulePath)
		if applicable {
			anyApplicable = true
		}
	}

	if !anyApplicable {
		return false, ""
	}

	// Check errors specific to this module
	hasErrors := false
	for _, err := range m.errors.GetErrors() {
		if err.ModuleID == moduleName && err.Level == pkg.Error {
			hasErrors = true
			break
		}
	}

	return hasErrors, lastTesterName
}

func (m *Manager) PrintResult() {
	green := color.New(color.FgGreen).SprintFunc()

	for _, result := range m.results {
		if result.skipped {
			continue
		}

		if result.failed {
			fmt.Fprintf(os.Stderr, "❌ [%s] %s\n", result.tester, result.name)
			m.printModuleErrors(result.name)
		} else {
			fmt.Printf("%s [%s] %s\n", green("✅"), result.tester, result.name)
		}
	}
}

func (m *Manager) printModuleErrors(moduleName string) {
	errs := getModuleErrors(m.errors.GetErrors(), moduleName)
	if len(errs) == 0 {
		return
	}

	w := new(tabwriter.Writer)

	const minWidth = 5

	buf := bytes.NewBuffer([]byte{})
	w.Init(buf, minWidth, 0, 0, ' ', 0)

	for idx := range errs {
		printErrorHeader(w, &errs[idx])
		printErrorBody(w, &errs[idx])

		fmt.Fprintln(w)
		w.Flush()
	}

	fmt.Fprint(os.Stderr, buf.String())
}

func getModuleErrors(errs []pkg.TestError, moduleName string) []pkg.TestError {
	filtered := make([]pkg.TestError, 0, len(errs))
	for idx := range errs {
		if errs[idx].ModuleID == moduleName {
			filtered = append(filtered, errs[idx])
		}
	}

	slices.SortFunc(filtered, func(a, b pkg.TestError) int {
		return cmp.Or(
			cmp.Compare(a.Level, b.Level),
			cmp.Compare(a.ModuleID, b.ModuleID),
			cmp.Compare(a.TestID, b.TestID),
		)
	})

	return filtered
}

func printErrorHeader(w *tabwriter.Writer, err *pkg.TestError) {
	blue := color.New(color.FgHiBlue).SprintFunc()

	fmt.Fprint(w, emoji.Sprintf(":monkey:"))
	fmt.Fprint(w, blue("["))

	if err.TestName != "" {
		fmt.Fprint(w, blue(err.TestName+" "))
	}

	fmt.Fprintf(w, "%s\n", color.New(color.FgHiBlue).SprintfFunc()("(#%s)]", err.TestID))
}

func printErrorBody(w *tabwriter.Writer, err *pkg.TestError) {
	msgColor := color.New(color.FgRed).SprintfFunc()

	fmt.Fprintf(w, "\t%s\t\t%s\n", "Message:", msgColor(prepareString(err.Text)))
	fmt.Fprintf(w, "\t%s\t\t%s\n", "Module:", err.ModuleID)

	if err.Expected != "" {
		fmt.Fprintf(w, "\t%s\t\t%s\n", "Expected:", prepareString(strings.TrimRight(err.Expected, "\n")))
	}

	if err.Got != "" {
		fmt.Fprintf(w, "\t%s\t\t%s\n", "Got:", prepareString(strings.TrimRight(err.Got, "\n")))
	}
}

func (m *Manager) HasCriticalErrors() bool {
	return m.errors.ContainsErrors()
}

func (m *Manager) registerTesters() {
	m.testers = []testers.Tester{
		tester.New(m.errors),
	}
}

func extractModuleName(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		return path[idx+1:]
	}
	return path
}

// prepareString handles a string and prepares it for tabwriter.
func prepareString(input string) string {
	const wrapLen = 100

	w := &strings.Builder{}

	split := strings.Split(wordwrap.WrapString(input, wrapLen), "\n")

	fmt.Fprint(w, strings.TrimSpace(split[0]))

	for i := 1; i < len(split); i++ {
		fmt.Fprintf(w, "\n\t\t\t%s", strings.TrimSpace(split[i]))
	}

	return w.String()
}
