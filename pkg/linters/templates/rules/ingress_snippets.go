/*
Copyright 2025 Flant JSC

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

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	IngressSnippetsRuleName = "ingress-snippets"
	nginxAnnotationPrefix   = "nginx.ingress.kubernetes.io/"

	// ingressNginxAnnotationsVersionHelper is the helm_lib switch that tells
	// whether the installed ingress-nginx module provides the additional
	// annotations introduced in v1.3.0. Snippet annotations may only remain in
	// module templates for controllers that do not have those annotations yet,
	// so every snippet must be behind a version check.
	ingressNginxAnnotationsVersionHelper = "helm_lib_module_ingress_nginx_annotations_version_ge"
	minIngressNginxAnnotationsVersion    = "1.3.0"
	// ingressNginxAnnotationsVersionLibHelm is the first lib_helm release that ships the helper.
	ingressNginxAnnotationsVersionLibHelm = "v1.72.30"
	snippetAnnotationsDocsURL             = "https://deckhouse.io/modules/ingress-nginx/alpha/faq.html#additional-ingress-annotations"
)

var (
	// snippetAnnotationRe matches any nginx.ingress.kubernetes.io/*-snippet
	// annotation key, including configuration, server, auth, modsecurity and
	// stream snippets.
	snippetAnnotationRe = regexp.MustCompile(regexp.QuoteMeta(nginxAnnotationPrefix) + `[A-Za-z0-9._-]+-snippet`)

	// helmActionRe matches a whole {{ ... }} Helm action, including actions
	// spread over multiple lines and both whitespace-trimming variants.
	helmActionRe = regexp.MustCompile(`(?s)\{\{-?\s*(.*?)\s*-?\}\}`)

	// annotationsVersionGuardRe matches the version-check helper call in a Helm
	// condition. The root context is normally $, but . is accepted as well.
	annotationsVersionGuardRe = regexp.MustCompile(
		`include\s+"` + regexp.QuoteMeta(ingressNginxAnnotationsVersionHelper) + `"\s+\(list\s+(?:\$|\.)\s+"([^"]+)"\s*\)`,
	)

	minAnnotationsVersion = semver.MustParse(minIngressNginxAnnotationsVersion)
)

type IngressSnippetsRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewIngressSnippetsRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *IngressSnippetsRule {
	return &IngressSnippetsRule{
		RuleMeta: pkg.RuleMeta{
			Name: IngressSnippetsRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
		},
		module:    m,
		errorList: errorList.WithRule(IngressSnippetsRuleName),
	}
}

var _ pkg.Rule = (*IngressSnippetsRule)(nil)

func (r *IngressSnippetsRule) Check(_ context.Context) {
	// Snippet annotations are checked in the raw templates rather than in the
	// rendered objects: a snippet behind a version conditional may not render
	// for the values used by dmt, but it still has to be guarded.
	checkedFiles := make(map[string]struct{})

	for _, object := range r.module.GetStorage() {
		if object.Unstructured.GetKind() != "Ingress" {
			continue
		}

		if object.AbsPath != "" {
			checkedFiles[object.AbsPath] = struct{}{}
		}
	}

	for path := range checkedFiles {
		r.checkSnippetAnnotations(path)
	}
}

func (r *IngressSnippetsRule) checkSnippetAnnotations(absPath string) {
	filePath := r.displayPath(absPath)

	if !r.Enabled(filePath) {
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		r.errorList.WithFilePath(filePath).Errorf("Failed to read file: %v", err)
		return
	}

	r.checkSnippetAnnotationsContent(filePath, string(content))
}

func (r *IngressSnippetsRule) displayPath(absPath string) string {
	modulePath := r.module.GetPath()
	if modulePath == "" {
		return absPath
	}

	relPath, err := filepath.Rel(modulePath, absPath)
	if err != nil {
		return absPath
	}

	return relPath
}

// helmBlock is one open Helm control-flow block. guarded reports whether the
// block is nested inside (or is itself) an if whose condition checks the
// ingress-nginx annotations version.
type helmBlock struct {
	kind    string
	guarded bool
}

func (r *IngressSnippetsRule) checkSnippetAnnotationsContent(filePath, content string) {
	stack := make([]helmBlock, 0, 8)
	last := 0

	for _, match := range helmActionRe.FindAllStringSubmatchIndex(content, -1) {
		r.reportUnguardedSnippetAnnotations(filePath, content, last, match[0], activeGuard(stack))

		action := content[match[2]:match[3]]
		updateHelmStack(&stack, action)

		last = match[1]
	}

	r.reportUnguardedSnippetAnnotations(filePath, content, last, len(content), activeGuard(stack))
}

func activeGuard(stack []helmBlock) bool {
	if len(stack) == 0 {
		return false
	}

	return stack[len(stack)-1].guarded
}

func updateHelmStack(stack *[]helmBlock, action string) {
	action = strings.TrimSpace(strings.Trim(action, "-"))
	if action == "" {
		return
	}

	fields := strings.Fields(action)
	if len(fields) == 0 {
		return
	}

	keyword := fields[0]

	switch keyword {
	case "if":
		*stack = append(*stack, helmBlock{
			kind:    keyword,
			guarded: activeGuard(*stack) || isAnnotationsVersionGuard(action),
		})
	case "with", "range", "define", "block":
		*stack = append(*stack, helmBlock{
			kind:    keyword,
			guarded: activeGuard(*stack),
		})
	case "else":
		if len(*stack) == 0 || (*stack)[len(*stack)-1].kind != "if" {
			return
		}

		// A snippet in the else branch is the common legacy path: render the
		// snippet only when the additional annotations are not available.
		top := &(*stack)[len(*stack)-1]
		top.guarded = top.guarded || isAnnotationsVersionGuard(action)
	case "end":
		if len(*stack) > 0 {
			*stack = (*stack)[:len(*stack)-1]
		}
	}
}

func isAnnotationsVersionGuard(action string) bool {
	for _, match := range annotationsVersionGuardRe.FindAllStringSubmatch(action, -1) {
		version, err := semver.NewVersion(match[1])
		if err == nil && !version.LessThan(minAnnotationsVersion) {
			return true
		}
	}

	return false
}

func (r *IngressSnippetsRule) reportUnguardedSnippetAnnotations(filePath, content string, start, end int, guarded bool) {
	if guarded || start >= end {
		return
	}

	segment := content[start:end]

	for _, loc := range snippetAnnotationRe.FindAllStringIndex(segment, -1) {
		lineStart := strings.LastIndex(segment[:loc[0]], "\n") + 1
		if strings.HasPrefix(strings.TrimSpace(segment[lineStart:]), "#") {
			continue
		}

		annotation := segment[loc[0]:loc[1]]
		line := strings.Count(content[:start+loc[0]], "\n") + 1

		r.errorList.WithFilePath(filePath).
			WithLineNumber(line).
			WithValue(annotation).
			Errorf("Ingress annotation %q is discouraged to use in DKP modules: snippet annotations may only be "+
				"used to support legacy versions of the ingress-nginx controller or in user namespaces when the "+
				"user explicitly allows them at the controller level. Enclose it in a Helm conditional "+
				"for %q with version %q or higher (requires lib_helm %s+), for example "+
				"{{- if include %q (list $ %q) }} ... {{- end }}, and migrate the directives to the supported "+
				"annotations: %s",
				annotation, ingressNginxAnnotationsVersionHelper, minIngressNginxAnnotationsVersion,
				ingressNginxAnnotationsVersionLibHelm, ingressNginxAnnotationsVersionHelper,
				minIngressNginxAnnotationsVersion, snippetAnnotationsDocsURL)
	}
}
