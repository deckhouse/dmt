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
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	DeprecatedHTTPRouteAnnotationsRuleName = "deprecated-httproute-annotations"
)

// deprecatedAnnotation is one annotation key that must no longer appear in
// module templates, along with the reason it was banned and a workaround
// snippet demonstrating the replacement, both surfaced in the finding.
type deprecatedAnnotation struct {
	Key        string
	Reason     string
	Workaround string
}

// deprecatedAnnotations is the list of annotations this rule flags. Add an entry
// here to ban another annotation; the scan and reporting are shared.
var deprecatedAnnotations = []deprecatedAnnotation{
	{
		Key:    "alb.network.deckhouse.io/response-headers-to-add",
		Reason: "deprecated in favor of the native Gateway API HTTPRoute ResponseHeaderModifier filter",
		Workaround: `  rules:
  - backendRefs: [...]
    matches: [...]
    filters:
    - type: ResponseHeaderModifier
      responseHeaderModifier:
        add:
        - name: Strict-Transport-Security
          value: max-age=31536000; includeSubDomains`,
	},
}

type DeprecatedHTTPRouteAnnotationsRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewDeprecatedHTTPRouteAnnotationsRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *DeprecatedHTTPRouteAnnotationsRule {
	return &DeprecatedHTTPRouteAnnotationsRule{
		RuleMeta: pkg.RuleMeta{
			Name: DeprecatedHTTPRouteAnnotationsRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
		},
		module:    m,
		errorList: errorList.WithRule(DeprecatedHTTPRouteAnnotationsRuleName),
	}
}

var _ pkg.Rule = (*DeprecatedHTTPRouteAnnotationsRule)(nil)

// Check scans every template file for the annotation keys in deprecatedAnnotations
// and reports each occurrence, regardless of whether the key appears as a plain
// YAML annotation or inside a Helm expression — the key text itself is what must
// no longer be used.
//
// The rule only runs when the module actually renders an Ingress, HTTPRoute, or
// ListenerSet: every entry in deprecatedAnnotations is specific to those
// resources, so a module with none of them has nothing for this check to say.
func (r *DeprecatedHTTPRouteAnnotationsRule) Check(_ context.Context) {
	m := r.module

	if !storageHasKind(m, "Ingress", "HTTPRoute", "ListenerSet") {
		return
	}

	templatesPath := filepath.Join(m.GetPath(), "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	files := fsutils.GetFiles(templatesPath, true, fsutils.FilterFileByExtensions(".yaml", ".yml", ".tpl"))

	for _, filePath := range files {
		relPath := fsutils.Rel(m.GetPath(), filePath)

		if !r.Enabled(relPath) {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			r.errorList.WithFilePath(relPath).Errorf("Failed to read file: %v", err)
			continue
		}

		r.checkContent(relPath, content)
	}
}

func (r *DeprecatedHTTPRouteAnnotationsRule) checkContent(relPath string, content []byte) {
	for _, annotation := range deprecatedAnnotations {
		re := regexp.MustCompile(regexp.QuoteMeta(annotation.Key))

		for _, loc := range re.FindAllIndex(content, -1) {
			line := strings.Count(string(content[:loc[0]]), "\n") + 1

			r.errorList.WithFilePath(relPath).
				WithLineNumber(line).
				WithValue(annotation.Key).
				Errorf("Annotation %q must not be used: %s. Add this filter to the relevant HTTPRoute rule instead:\n%s",
					annotation.Key, annotation.Reason, annotation.Workaround)
		}
	}
}
