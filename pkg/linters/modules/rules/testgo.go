/*
Copyright 2021 Flant JSC

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

package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/d8-lint/pkg/errors"
)

const (
	ginkgoImport        = `. "github.com/onsi/ginkgo"`
	gomegaImport        = `. "github.com/onsi/gomega"`
	commonTestGoContent = `
func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}
`
)

func CommonTestGoForHooks(name, path string) *errors.LintRuleError {
	if !IsExistsOnFilesystem(path, HooksDir) {
		return nil
	}

	if matches, _ := filepath.Glob(filepath.Join(path, HooksDir, "*.go")); len(matches) == 0 {
		return nil
	}

	commonTestPath := filepath.Join(path, HooksDir, "common_test.go")
	if !IsExistsOnFilesystem(commonTestPath) {
		return errors.NewLintRuleError(
			ID,
			name,
			ModuleLabel(name),
			nil,
			"Module does not contain %q file", commonTestPath,
		)
	}

	contentBytes, err := os.ReadFile(commonTestPath)
	if err != nil {
		return errors.NewLintRuleError(
			ID,
			name,
			ModuleLabel(name),
			nil,
			"Module does not contain %q file", commonTestPath,
		)
	}

	var errs []string
	if !strings.Contains(string(contentBytes), commonTestGoContent) {
		errs = append(errs,
			fmt.Sprintf("Module content of %q file does not contain:\n\t%s", commonTestPath, commonTestGoContent),
		)
	}

	if !strings.Contains(string(contentBytes), gomegaImport) {
		errs = append(errs,
			fmt.Sprintf("Module content of %q file does not contain:\n\t%s", commonTestPath, gomegaImport),
		)
	}

	if !strings.Contains(string(contentBytes), ginkgoImport) {
		errs = append(errs,
			fmt.Sprintf("Module content of %q file does not contain:\n\t%s", commonTestPath, ginkgoImport),
		)
	}

	if len(errs) > 0 {
		errstr := strings.Join(errs, "\n")

		return errors.NewLintRuleError(
			ID,
			name,
			ModuleLabel(name),
			nil,
			"%v",
			errstr,
		)
	}

	return nil
}
