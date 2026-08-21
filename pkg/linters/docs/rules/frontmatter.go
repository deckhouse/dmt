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

package rules

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const FrontMatterRuleName = "front-matter"

// utf8BOM is the UTF-8 byte-order mark, stripped from a file before looking for
// the leading front matter delimiter. Spelled out as bytes so the source file
// itself carries no BOM.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func NewFrontMatterRule() *FrontMatterRule {
	return &FrontMatterRule{
		RuleMeta: pkg.RuleMeta{
			Name: FrontMatterRuleName,
		},
	}
}

type FrontMatterRule struct {
	pkg.RuleMeta
	pkg.PathRule
}

// CheckFiles validates the YAML front matter of markdown files under docs/.
//
// Rationale: the documentation site is rendered (Hugo) from the module's docs/
// tree, and a markdown file whose YAML front matter is malformed — an
// unterminated "---" block or invalid YAML between the delimiters — aborts that
// render. markdownlint does not parse front matter as YAML and no-lang-key only
// looks for a "lang:" key, so a broken front matter is caught nowhere at lint
// time and only surfaces later as an opaque site-build failure. This rule shifts
// that detection left: the broken file is reported here, with its path, before
// the module is released.
//
// Scope mirrors the render: the whole docs/ tree is scanned recursively, except
// docs/internal/ which is not rendered (the same subtree the size rule excludes).
// Only files that actually begin with a "---" front matter delimiter are checked;
// a file without front matter has nothing to validate and is skipped.
func (r *FrontMatterRule) CheckFiles(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	modulePath := m.GetPath()
	if modulePath == "" {
		return
	}

	docsPath := filepath.Join(modulePath, "docs")
	if _, err := os.Stat(docsPath); err != nil {
		return
	}

	files := fsutils.GetFiles(docsPath, false, fsutils.FilterFileByExtensions(".md"))

	for _, fileName := range files {
		relToDocs := fsutils.Rel(docsPath, fileName)
		// docs/internal/ is not rendered as documentation (same exclusion as the
		// size rule), so a broken front matter there cannot break the site build.
		if relToDocs == "internal" || strings.HasPrefix(relToDocs, "internal"+string(filepath.Separator)) {
			continue
		}

		relFromModule := fsutils.Rel(modulePath, fileName)
		if !r.Enabled(relFromModule) {
			continue
		}

		r.checkFile(modulePath, fileName, errorList)
	}
}

func (r *FrontMatterRule) checkFile(modulePath, fileName string, errorList *errors.LintRuleErrorsList) {
	relPath := fsutils.Rel(modulePath, fileName)

	content, err := os.ReadFile(fileName)
	if err != nil {
		errorList.WithFilePath(relPath).WithValue(err.Error()).Error("failed to read file")

		return
	}

	// Hugo only treats a leading "---" (the very first line, after an optional
	// UTF-8 BOM) as YAML front matter. Anything else — including a "---" thematic
	// break in the body — is not front matter and must not be parsed as YAML.
	text := string(bytes.TrimPrefix(content, utf8BOM))
	lines := strings.Split(text, "\n")

	if len(lines) == 0 || !frontMatterDelimiter.MatchString(lines[0]) {
		return
	}

	endIdx := -1

	for i := 1; i < len(lines); i++ {
		if frontMatterDelimiter.MatchString(lines[i]) {
			endIdx = i

			break
		}
	}

	if endIdx == -1 {
		errorList.WithFilePath(relPath).
			Errorf("unterminated YAML front matter: the block opened by %q on line 1 is never closed by a matching %q", "---", "---")

		return
	}

	block := strings.Join(lines[1:endIdx], "\n")

	var value any
	if err := yaml.Unmarshal([]byte(block), &value); err != nil {
		errorList.WithFilePath(relPath).
			Errorf("invalid YAML front matter:\n%s", err)

		return
	}

	// Hugo parses a "---" block as a YAML mapping (key: value metadata). Content
	// that is valid YAML but not a mapping — a bare scalar or a sequence — is
	// reported only as a warning (Warnf), not at the rule's error level: it is the
	// softest, most false-positive-prone signal (e.g. an author who opened "---"
	// for something other than metadata), so it must not fail CI. A syntax error
	// inside the block and an unterminated block stay at the rule's level (error by
	// default), because those are unambiguous breakages. An empty block (nil) is
	// allowed.
	if value == nil {
		return
	}

	if _, ok := value.(map[string]any); !ok {
		errorList.WithFilePath(relPath).
			Warnf("front matter between %q delimiters must be a YAML mapping (key: value metadata), got %T", "---", value)
	}
}
