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
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// kindLineRe returns a regexp matching a manifest's `kind:` field on its own
// line for any of the given kinds — the same shape a rendered Kubernetes YAML
// document uses, regardless of the Helm expressions around it. It is used to
// test "does this template file ever emit an object of this kind" without
// rendering the chart.
func kindLineRe(kinds ...string) *regexp.Regexp {
	escaped := make([]string, len(kinds))
	for i, k := range kinds {
		escaped[i] = regexp.QuoteMeta(k)
	}

	return regexp.MustCompile(`(?m)^kind:\s*(` + strings.Join(escaped, "|") + `)\s*$`)
}

// storageHasKind reports whether module's rendered objects include at least one
// of the given kinds. It gates the enablement/annotation rules so they only run
// on modules that actually ship the kind of resource they check — a module with
// no Ingress has nothing for ingress-enablement to say, and likewise for Gateway
// API and HTTPRoute/ListenerSet.
func storageHasKind(m pkg.Module, kinds ...string) bool {
	for _, object := range m.GetStorage() {
		kind := object.Unstructured.GetKind()

		for _, k := range kinds {
			if kind == k {
				return true
			}
		}
	}

	return false
}

// checkKindGatedByHelper scans every template file of module for kindRe and
// reports each file that matches it but never mentions helperName anywhere in
// the same file. kindLabel names the resource kind(s) in the finding text,
// and valuesHint names the concrete `.Values` path(s) an author would set to
// control it, so the finding says exactly what to change, not just which
// helper to call.
//
// This is a textual, same-file heuristic: it does not verify that helperName
// actually gates the specific manifest kindRe matched, only that both appear
// somewhere in the same file. See IngressEnablementRule.Check for why that
// trade-off was chosen over a full Helm-template control-flow parser.
func checkKindGatedByHelper(
	m pkg.Module,
	errorList *errors.LintRuleErrorsList,
	pathRule pkg.PathRule,
	kindRe *regexp.Regexp,
	helperName string,
	kindLabel string,
	valuesHint string,
) {
	templatesPath := filepath.Join(m.GetPath(), "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	files := fsutils.GetFiles(templatesPath, true, fsutils.FilterFileByExtensions(".yaml", ".yml", ".tpl"))
	helperBytes := []byte(helperName)

	for _, filePath := range files {
		relPath := fsutils.Rel(m.GetPath(), filePath)

		if !pathRule.Enabled(relPath) {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			errorList.WithFilePath(relPath).Errorf("Failed to read file: %v", err)
			continue
		}

		loc := kindRe.FindIndex(content)
		if loc == nil {
			continue
		}

		if bytes.Contains(content, helperBytes) {
			continue
		}

		line := bytes.Count(content[:loc[0]], []byte("\n")) + 1

		errorList.WithFilePath(relPath).
			WithLineNumber(line).
			Errorf(
				"File creates a %s object but never checks %q, so its creation cannot be "+
					"controlled via %s. Guard the manifest with "+
					"{{- if eq (include %q .) \"true\" }} ... {{- end }} (requires lib_helm "+
					"v1.72.21+) so it can be disabled the same way every other module's %s does.",
				kindLabel, helperName, valuesHint, helperName, kindLabel,
			)
	}
}
