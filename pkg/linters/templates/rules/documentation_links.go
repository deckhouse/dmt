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
	DocumentationLinksRuleName = "documentation-links"
)

var (
	// publicDocumentationURLRe matches a link to the public documentation site.
	// The path stops at characters that end a link in Markdown, YAML, JSON or a
	// Go template action.
	publicDocumentationURLRe = regexp.MustCompile("https?://(?:www\\.)?deckhouse\\.(?:io|ru)(?:/[^\\s)\\]\"'<>{}`|]*)?")

	// multilineTemplateActionRe matches a Go template action, e.g. {{- else }},
	// including actions that span several lines, unlike templateActionRe.
	multilineTemplateActionRe = regexp.MustCompile(`(?s){{-?\s*(.*?)\s*-?}}`)

	// alertsAndDashboardsKindRe matches a manifest of a resource that carries
	// alerts or dashboards, so that only such files under templates/ are scanned.
	alertsAndDashboardsKindRe = regexp.MustCompile(`(?m)^\s*kind:\s*["']?(?:` + strings.Join([]string{
		"PrometheusRule",
		"CustomPrometheusRules",
		"GrafanaDashboardDefinition",
		"ClusterObservabilityMetricsRulesGroup",
		"ClusterObservabilityPropagatedMetricsRulesGroup",
		"ObservabilityMetricsRulesGroup",
		"ClusterObservabilityLogsRulesGroup",
		"ObservabilityLogsRulesGroup",
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
		fallbacks = publicDomainFallbackRanges(content)
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

// templateBlock is an open {{ if }}, {{ range }}, {{ with }}, {{ define }} or
// {{ block }} action.
type templateBlock struct {
	// fallbackBranch is the branch rendered when publicDomainTemplate is not set:
	// "else" for {{ if ...publicDomainTemplate }}, "then" for
	// {{ if not ...publicDomainTemplate }}, and "" when the condition does not
	// depend on publicDomainTemplate.
	fallbackBranch string
	inElse         bool
	start          int
}

func (b *templateBlock) inFallback() bool {
	return b.fallbackBranch == "else" && b.inElse || b.fallbackBranch == "then" && !b.inElse
}

// publicDomainFallbackRanges returns the byte ranges of the branches rendered
// only when publicDomainTemplate is not set, where linking to the public site is
// the correct fallback:
//
//	{{ if .Values.global.modules.publicDomainTemplate }}...in-cluster link...{{ else }}https://deckhouse.io{{ end }}
func publicDomainFallbackRanges(content string) [][2]int {
	var (
		ranges [][2]int
		stack  []*templateBlock
	)

	for _, loc := range multilineTemplateActionRe.FindAllStringSubmatchIndex(content, -1) {
		fields := strings.Fields(content[loc[2]:loc[3]])
		if len(fields) == 0 {
			continue
		}

		keyword, rest := fields[0], strings.Join(fields[1:], " ")

		switch keyword {
		case "if":
			block := &templateBlock{start: loc[1]}
			if strings.Contains(rest, "publicDomainTemplate") {
				block.fallbackBranch = "else"
				if strings.HasPrefix(strings.TrimLeft(rest, "( "), "not ") {
					block.fallbackBranch = "then"
				}
			}

			stack = append(stack, block)
		case "range", "with", "define", "block":
			stack = append(stack, &templateBlock{start: loc[1]})
		case "else":
			if len(stack) == 0 {
				continue
			}

			block := stack[len(stack)-1]
			if block.inFallback() {
				ranges = append(ranges, [2]int{block.start, loc[0]})
			}

			block.inElse = true
			block.start = loc[1]
		case "end":
			if len(stack) == 0 {
				continue
			}

			block := stack[len(stack)-1]
			if block.inFallback() {
				ranges = append(ranges, [2]int{block.start, loc[0]})
			}

			stack = stack[:len(stack)-1]
		}
	}

	return ranges
}

func inRanges(offset int, ranges [][2]int) bool {
	for _, r := range ranges {
		if offset >= r[0] && offset < r[1] {
			return true
		}
	}

	return false
}
