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
	"slices"
	"strings"
	"text/template/parse"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	DocumentationLinksRuleName = "documentation-links"
)

var (
	// publicDocumentationURLRe matches a link to the public documentation site.
	// The path stops at characters that end a link in Markdown, YAML, JSON or a
	// Go template action.
	publicDocumentationURLRe = regexp.MustCompile("https?://(?:www\\.)?deckhouse\\.(?:io|ru)(?:/[^\\s)\\]\"'<>{}`|]*)?")

	// alertsAndDashboardsKindRe matches a manifest of a resource that carries
	// alerts or dashboards, so that only such files under templates/ are scanned.
	alertsAndDashboardsKindRe = regexp.MustCompile(`(?m)^\s*kind:\s*["']?(?:` + strings.Join([]string{
		"PrometheusRule",
		"CustomPrometheusRules",
		"GrafanaDashboardDefinition",
		"ClusterObservabilityMetricsRulesGroup",
		"ClusterObservabilityPropagatedMetricsRulesGroup",
		"ObservabilityMetricsRulesGroup",
		"ClusterObservabilityDashboard",
		"ClusterObservabilityPropagatedDashboard",
		"ObservabilityDashboard",
	}, "|") + `)["']?\s*$`)
)

// DocumentationLinksRule flags links to the public documentation site in module
// alerts and dashboards. Clusters in closed environments cannot reach the public
// site, so such links must point to the in-cluster documentation instead.
type DocumentationLinksRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewDocumentationLinksRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *DocumentationLinksRule {
	return &DocumentationLinksRule{
		RuleMeta: pkg.RuleMeta{
			Name: DocumentationLinksRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
		},
		module:    m,
		errorList: errorList.WithRule(DocumentationLinksRuleName),
	}
}

var _ pkg.Rule = (*DocumentationLinksRule)(nil)

// documentationLinksSource is a set of files the rule scans.
type documentationLinksSource struct {
	dir        string
	extensions []string
	// onlyAlertsAndDashboards limits the scan to manifests of alert and dashboard
	// resources; other templates may legitimately link to the public site.
	onlyAlertsAndDashboards bool
}

var documentationLinksSources = []documentationLinksSource{
	{dir: filepath.Join("monitoring", "prometheus-rules"), extensions: []string{".yaml", ".yml", ".tpl"}},
	{dir: filepath.Join("monitoring", "grafana-dashboards"), extensions: []string{".json", ".tpl"}},
	{dir: "templates", extensions: []string{".yaml", ".yml", ".tpl"}, onlyAlertsAndDashboards: true},
}

// Check scans source files rather than rendered objects: findings get exact
// line numbers, and plain .yaml rules and .json dashboards — which lib-helm does
// not pass through tpl — are covered as well.
func (r *DocumentationLinksRule) Check(_ context.Context) {
	m := r.module

	for _, source := range documentationLinksSources {
		dir := filepath.Join(m.GetPath(), source.dir)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		files := fsutils.GetFiles(dir, true, fsutils.FilterFileByExtensions(source.extensions...))

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

			if source.onlyAlertsAndDashboards && !alertsAndDashboardsKindRe.Match(content) {
				continue
			}

			r.checkContent(relPath, string(content))
		}
	}
}

func (r *DocumentationLinksRule) checkContent(relPath, content string) {
	rendered := isRenderedByHelm(relPath)

	var fallbacks [][2]int
	if rendered {
		fallbacks = publicSiteFallbackRanges(content)
	}

	for _, loc := range publicDocumentationURLRe.FindAllStringIndex(content, -1) {
		if continuesHostname(content, loc[1]) || inRanges(loc[0], fallbacks) {
			continue
		}

		link := content[loc[0]:loc[1]]
		line := strings.Count(content[:loc[0]], "\n") + 1

		msg := "Link %q points to the public documentation site, which is unreachable from closed environments. " +
			"Link to the in-cluster documentation instead: " +
			`{{ include "helm_lib_module_documentation_uri" (list . %q) }}`
		if !rendered {
			msg += ". This file is not rendered by Helm: rename it to .tpl and escape the existing " +
				"Prometheus/Grafana templates, e.g. {{`{{ $labels.node }}`}}"
		}

		r.errorList.WithFilePath(relPath).
			WithLineNumber(line).
			WithValue(link).
			Errorf(msg, link, documentationPath(link))
	}
}

// isRenderedByHelm reports whether Helm template actions in the file are
// executed. Everything under templates/ is rendered, while lib-helm passes only
// .tpl files from monitoring/ through tpl.
func isRenderedByHelm(relPath string) bool {
	return strings.HasPrefix(filepath.ToSlash(relPath), "templates/") || filepath.Ext(relPath) == ".tpl"
}

// continuesHostname reports whether the match is only a prefix of a longer
// hostname, e.g. https://deckhouse.io.example.com.
func continuesHostname(content string, end int) bool {
	rest := content[end:]
	if rest == "" {
		return false
	}

	c := rest[0]
	if c == '-' || isAlphanumeric(c) {
		return true
	}

	return c == '.' && len(rest) > 1 && isAlphanumeric(rest[1])
}

func isAlphanumeric(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// documentationPath returns the part of the link after the host, which is what
// helm_lib_module_documentation_uri expects.
func documentationPath(link string) string {
	withoutScheme := link[strings.Index(link, "://")+len("://"):]
	if i := strings.Index(withoutScheme, "/"); i >= 0 {
		return withoutScheme[i:]
	}

	return "/"
}

var (
	publicDomainTemplatePath = []string{"Values", "global", "modules", "publicDomainTemplate"}
	enabledModulesPath       = []string{"Values", "global", "enabledModules"}
)

// publicSiteFallbackRanges parses the file as a Go template, the way Helm does,
// and returns the byte ranges of the text rendered only when the in-cluster
// documentation is unavailable, where linking to the public site is the correct
// fallback:
//
//	{{ if .Values.global.modules.publicDomainTemplate }}...in-cluster link...{{ else }}https://deckhouse.io{{ end }}
//
// A file that does not parse has no fallbacks: Helm cannot render it either.
func publicSiteFallbackRanges(content string) [][2]int {
	tree := parse.New("documentation-links")
	// Helm and sprig functions are not known here; only the structure matters.
	tree.Mode = parse.SkipFuncCheck

	trees := map[string]*parse.Tree{}
	if _, err := tree.Parse(content, "", "", trees); err != nil {
		return nil
	}

	var ranges [][2]int

	// Every {{ define }} is a separate tree; positions are offsets in content.
	for _, t := range trees {
		collectFallbackRanges(t.Root, false, &ranges)
	}

	return ranges
}

// collectFallbackRanges walks the template tree and records the text inside
// fallback branches. An {{ else if }} chain is parsed as an if nested in the
// else branch, so the branches after a fallback condition stay fallbacks.
func collectFallbackRanges(node parse.Node, fallback bool, ranges *[][2]int) {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}

		for _, child := range n.Nodes {
			collectFallbackRanges(child, fallback, ranges)
		}
	case *parse.TextNode:
		if fallback {
			*ranges = append(*ranges, [2]int{int(n.Pos), int(n.Pos) + len(n.Text)})
		}
	case *parse.IfNode:
		collectBranchRanges(&n.BranchNode, fallback, ranges)
	case *parse.WithNode:
		collectBranchRanges(&n.BranchNode, fallback, ranges)
	case *parse.RangeNode:
		collectFallbackRanges(n.List, fallback, ranges)
		collectFallbackRanges(n.ElseList, fallback, ranges)
	}
}

func collectBranchRanges(branch *parse.BranchNode, fallback bool, ranges *[][2]int) {
	collectFallbackRanges(branch.List, fallback || documentationUnavailableWhen(branch.Pipe, true), ranges)
	collectFallbackRanges(branch.ElseList, fallback || documentationUnavailableWhen(branch.Pipe, false), ranges)
}

// documentationUnavailableWhen reports whether the pipeline being truthy (or
// falsy) guarantees that the in-cluster documentation is unavailable, that is
// publicDomainTemplate is empty or the documentation module is disabled. It
// follows not, empty, comparisons with "", and, or and parentheses; any other
// expression is an opaque value.
func documentationUnavailableWhen(pipe *parse.PipeNode, truthy bool) bool {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return false
	}

	// In a | f b the value of a is the last argument of f.
	last := pipe.Cmds[len(pipe.Cmds)-1]
	args := slices.Clone(last.Args)

	if len(pipe.Cmds) > 1 {
		args = append(args, &parse.PipeNode{NodeType: parse.NodePipe, Cmds: pipe.Cmds[:len(pipe.Cmds)-1]})
	}

	return commandUnavailableWhen(args, truthy)
}

func commandUnavailableWhen(args []parse.Node, truthy bool) bool {
	if len(args) == 1 {
		return valueUnavailableWhen(args[0], truthy)
	}

	fn, ok := args[0].(*parse.IdentifierNode)
	if !ok {
		return false
	}

	operands := args[1:]

	switch fn.Ident {
	case "not", "empty":
		if len(operands) == 1 {
			return valueUnavailableWhen(operands[0], !truthy)
		}
	case "eq":
		if len(operands) == 2 && isStringNode(operands[1], "") {
			return valueUnavailableWhen(operands[0], !truthy)
		}

		if len(operands) == 2 && isStringNode(operands[0], "") {
			return valueUnavailableWhen(operands[1], !truthy)
		}
	case "and":
		// A truthy "and" needs one operand that implies unavailability, a
		// falsy one needs every operand to.
		if truthy {
			return anyOperandUnavailableWhen(operands, truthy)
		}

		return allOperandsUnavailableWhen(operands, truthy)
	case "or":
		if truthy {
			return allOperandsUnavailableWhen(operands, truthy)
		}

		return anyOperandUnavailableWhen(operands, truthy)
	case "has":
		// has "documentation" .Values.global.enabledModules
		return !truthy && len(operands) == 2 &&
			isStringNode(operands[0], "documentation") && isValuesPath(operands[1], enabledModulesPath)
	}

	return false
}

func valueUnavailableWhen(node parse.Node, truthy bool) bool {
	if pipe, ok := node.(*parse.PipeNode); ok {
		return documentationUnavailableWhen(pipe, truthy)
	}

	return !truthy && isValuesPath(node, publicDomainTemplatePath)
}

func anyOperandUnavailableWhen(operands []parse.Node, truthy bool) bool {
	return slices.ContainsFunc(operands, func(operand parse.Node) bool {
		return valueUnavailableWhen(operand, truthy)
	})
}

func allOperandsUnavailableWhen(operands []parse.Node, truthy bool) bool {
	return len(operands) > 0 && !slices.ContainsFunc(operands, func(operand parse.Node) bool {
		return !valueUnavailableWhen(operand, truthy)
	})
}

// isValuesPath reports whether the node is the given field of the root
// context, written as .Values.x or $.Values.x.
func isValuesPath(node parse.Node, path []string) bool {
	switch n := node.(type) {
	case *parse.PipeNode:
		// The value piped into a function, e.g. .Values.global.enabledModules
		// in .Values.global.enabledModules | has "documentation".
		return len(n.Cmds) == 1 && len(n.Cmds[0].Args) == 1 && isValuesPath(n.Cmds[0].Args[0], path)
	case *parse.FieldNode:
		return slices.Equal(n.Ident, path)
	case *parse.VariableNode:
		return len(n.Ident) > 0 && n.Ident[0] == "$" && slices.Equal(n.Ident[1:], path)
	}

	return false
}

func isStringNode(node parse.Node, text string) bool {
	s, ok := node.(*parse.StringNode)
	return ok && s.Text == text
}

func inRanges(offset int, ranges [][2]int) bool {
	for _, r := range ranges {
		if offset >= r[0] && offset < r[1] {
			return true
		}
	}

	return false
}
