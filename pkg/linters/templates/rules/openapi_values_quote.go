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
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/modules/values"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// OpenAPIValuesQuoteRuleName is the rule id reported in findings and used as the
// config key (templates.rules.openapi-values-quote / exclude-rules key).
const OpenAPIValuesQuoteRuleName = "openapi-values-quote"

// openAPIValuesFiles are the module value-schema files that define the
// .Values.<module> subtree consumed by templates.
var openAPIValuesFiles = []string{"values.yaml", "config-values.yaml"}

// safeFuncs are Helm/sprig pipeline functions whose output is guaranteed to be a
// YAML-safe scalar — either already quoted (quote/squote/toJson) or restricted to
// an alphabet that never needs quoting (base-N encoders, checksums). A value piped
// through one of them does not need an explicit quote.
var safeFuncs = map[string]struct{}{
	"quote": {}, "squote": {},
	"toJson": {}, "mustToJson": {}, "toRawJson": {}, "mustToRawJson": {},
	"b64enc": {}, "b32enc": {},
	"sha1sum": {}, "sha256sum": {}, "sha512sum": {}, "adler32sum": {},
}

// templateActionRe matches a single Go-template action {{ ... }}, capturing the
// inner expression with the surrounding whitespace/`-` trim markers stripped.
var templateActionRe = regexp.MustCompile(`{{-?\s*(.*?)\s*-?}}`)

// blockScalarHeaderRe matches a YAML block-scalar header line (`key: |`, `- >-`,
// `key: |+2 # x`, …). Content indented under such a header is a literal string, so
// substitutions there are already part of a string and must not be quoted.
var blockScalarHeaderRe = regexp.MustCompile(`^(\s*)(-\s+)?([\w.\-/]+\s*:\s*)?[|>][+-]?\d*\s*(#.*)?$`)

// standaloneOpenRe matches the text before an action when the action is the whole
// YAML value: indentation, an optional list dash, an optional `key:`, and an
// optional opening quote (captured).
var standaloneOpenRe = regexp.MustCompile(`^\s*(-\s+)?([\w.\-/]+\s*:\s*)?(["']?)\s*$`)

// OpenAPIValuesQuoteRule flags template usages of module OpenAPI string values that
// have no validation pattern (no pattern/enum/format) and are rendered unquoted.
// Such values can contain characters that break YAML or silently change the parsed
// type, so they must be quoted in templates.
type OpenAPIValuesQuoteRule struct {
	pkg.RuleMeta
	pkg.StringRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*OpenAPIValuesQuoteRule)(nil)

func NewOpenAPIValuesQuoteRule(excludeRules []pkg.StringRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *OpenAPIValuesQuoteRule {
	return &OpenAPIValuesQuoteRule{
		RuleMeta: pkg.RuleMeta{
			Name: OpenAPIValuesQuoteRuleName,
		},
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(OpenAPIValuesQuoteRuleName),
	}
}

// riskyPaths holds the value paths (relative to the module values root) whose type
// resolves to an unvalidated string. scalar paths are string leaves; array paths are
// arrays whose items are unvalidated strings (checked through `range`).
type riskyPaths struct {
	scalar map[string]struct{}
	array  map[string]struct{}
}

func newRiskyPaths() *riskyPaths {
	return &riskyPaths{
		scalar: make(map[string]struct{}),
		array:  make(map[string]struct{}),
	}
}

func (p *riskyPaths) empty() bool {
	return len(p.scalar) == 0 && len(p.array) == 0
}

// Check parses the module OpenAPI value schema, collects the pattern-less string
// paths, and reports every unquoted usage of them in templates.
func (r *OpenAPIValuesQuoteRule) Check(_ context.Context) {
	m, errorList := r.module, r.errorList

	modulePath := m.GetPath()
	valuesKey := values.ModuleCamelName(m.GetName())

	risky := newRiskyPaths()

	for _, name := range openAPIValuesFiles {
		data, err := os.ReadFile(filepath.Join(modulePath, "openapi", name))
		if err != nil {
			continue
		}

		schema := make(map[string]any)
		if err := yaml.Unmarshal(data, &schema); err != nil {
			// A malformed schema is reported by the openapi linter; skip it here.
			continue
		}

		(&schemaWalker{root: schema, out: risky}).walk("", schema, 0)
	}

	if risky.empty() {
		return
	}

	templatesPath := filepath.Join(modulePath, "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	files := fsutils.GetFiles(templatesPath, true,
		fsutils.FilterFileByExtensions(".yaml", ".yml", ".tpl", ".tpl.yaml", ".tpl.yml"))

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		relPath := fsutils.Rel(modulePath, filePath)
		lines := strings.Split(string(content), "\n")
		inBlock := computeBlockScalarLines(lines)

		r.checkScalars(relPath, lines, inBlock, valuesKey, risky, errorList)
		r.checkArrayRanges(relPath, lines, inBlock, valuesKey, risky, errorList)
	}
}

// checkScalars reports unquoted standalone emissions of scalar string values.
func (r *OpenAPIValuesQuoteRule) checkScalars(
	relPath string, lines []string, inBlock []bool, valuesKey string,
	risky *riskyPaths, errorList *errors.LintRuleErrorsList,
) {
	prefix := ".Values." + valuesKey + "."

	for i, line := range lines {
		if inBlock[i] {
			continue
		}

		for _, loc := range templateActionRe.FindAllStringSubmatchIndex(line, -1) {
			inner := strings.TrimSpace(line[loc[2]:loc[3]])

			cmd := leadingCommand(inner)
			if !strings.HasPrefix(cmd, prefix) {
				continue
			}

			valPath := cmd[len(prefix):]
			if _, ok := risky.scalar[valPath]; !ok {
				continue
			}

			if !r.Enabled(valPath) {
				continue
			}

			if pipelineIsSafe(inner) {
				continue
			}

			if standalone, quoted := analyzeContext(line[:loc[0]], line[loc[1]:]); !standalone || quoted {
				continue
			}

			errorList.WithFilePath(relPath).WithLineNumber(i+1).
				Errorf("value '.Values.%s.%s' is an OpenAPI string without a validation pattern (pattern/enum/format) and must be quoted in templates: use '| quote' or wrap the value in quotes",
					valuesKey, valPath)
		}
	}
}

// arrayScope tracks a template block while scanning for `range` loops over risky
// string arrays. elemVar is the loop element variable ("$x" or "." for dot-binding)
// when the range iterates a risky array; it is empty for any other block.
type arrayScope struct {
	rebindDot bool
	elemVar   string
	arrPath   string
}

// checkArrayRanges reports unquoted emissions of elements while ranging over a risky
// string array (`{{- range $e := .Values.mod.list }}{{ $e }}{{- end }}`).
func (r *OpenAPIValuesQuoteRule) checkArrayRanges(
	relPath string, lines []string, inBlock []bool, valuesKey string,
	risky *riskyPaths, errorList *errors.LintRuleErrorsList,
) {
	if len(risky.array) == 0 {
		return
	}

	prefix := ".Values." + valuesKey + "."

	var stack []arrayScope

	innermostDotScope := func() *arrayScope {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].rebindDot {
				return &stack[i]
			}
		}

		return nil
	}

	findVarScope := func(v string) *arrayScope {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].elemVar == v {
				return &stack[i]
			}
		}

		return nil
	}

	for i, line := range lines {
		for _, loc := range templateActionRe.FindAllStringSubmatchIndex(line, -1) {
			inner := strings.TrimSpace(line[loc[2]:loc[3]])

			switch firstToken(inner) {
			case "end":
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}

				continue
			case "else":
				continue
			case "if":
				stack = append(stack, arrayScope{rebindDot: false})
				continue
			case "with", "define", "block":
				stack = append(stack, arrayScope{rebindDot: true})
				continue
			case "range":
				stack = append(stack, newRangeScope(inner, prefix, risky, r))
				continue
			}

			tracked := emissionScope(leadingCommand(inner), innermostDotScope, findVarScope)
			if tracked == nil {
				continue
			}

			if inBlock[i] || pipelineIsSafe(inner) {
				continue
			}

			if standalone, quoted := analyzeContext(line[:loc[0]], line[loc[1]:]); !standalone || quoted {
				continue
			}

			errorList.WithFilePath(relPath).WithLineNumber(i+1).
				Errorf("array element from '.Values.%s.%s' is an OpenAPI string without a validation pattern (pattern/enum/format) and must be quoted in templates: use '| quote' or wrap the value in quotes",
					valuesKey, tracked.arrPath)
		}
	}
}

// newRangeScope builds the scope for a `range` action, marking it as a tracked risky
// array iteration when it ranges over a risky array path that is not excluded.
func newRangeScope(inner, prefix string, risky *riskyPaths, rule *OpenAPIValuesQuoteRule) arrayScope {
	sc := arrayScope{rebindDot: true}

	coll, elemVar := parseRange(inner)

	collCmd := firstToken(coll)
	if !strings.HasPrefix(collCmd, prefix) {
		return sc
	}

	arr := collCmd[len(prefix):]
	if _, ok := risky.array[arr]; !ok || !rule.Enabled(arr) {
		return sc
	}

	sc.elemVar = elemVar
	sc.arrPath = arr

	return sc
}

// emissionScope returns the tracked range scope an emission of cmd refers to, or nil.
// A bare dot resolves to the innermost dot-rebinding scope; a `$var` resolves to the
// nearest enclosing scope that introduced it.
func emissionScope(cmd string, innermostDot func() *arrayScope, findVar func(string) *arrayScope) *arrayScope {
	switch {
	case cmd == ".":
		if s := innermostDot(); s != nil && s.elemVar == "." {
			return s
		}
	case strings.HasPrefix(cmd, "$"):
		if s := findVar(cmd); s != nil && s.elemVar == cmd {
			return s
		}
	}

	return nil
}

// parseRange splits a `range ...` action into the collection expression and the loop
// element variable. `range $i, $e := X` -> ("X", "$e"); `range $e := X` -> ("X", "$e");
// `range X` -> ("X", ".").
func parseRange(inner string) (string, string) {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(inner), "range"))

	if left, right, found := strings.Cut(rest, ":="); found {
		vars := strings.Split(left, ",")
		return strings.TrimSpace(right), strings.TrimSpace(vars[len(vars)-1])
	}

	return rest, "."
}

// leadingCommand returns the first token of the first stage of a template pipeline —
// the command that produces the value (`.Values.x`, `if`, `range`, `$v`, `quote`, …).
func leadingCommand(inner string) string {
	stages := splitPipeline(inner)
	if len(stages) == 0 {
		return ""
	}

	return firstToken(stages[0])
}

// pipelineIsSafe reports whether any function stage of the pipeline is known to emit
// a YAML-safe scalar (see safeFuncs).
func pipelineIsSafe(inner string) bool {
	stages := splitPipeline(inner)
	for _, stage := range stages[1:] {
		if _, ok := safeFuncs[firstToken(stage)]; ok {
			return true
		}
	}

	return false
}

// splitPipeline splits a template expression on top-level `|`, ignoring pipes inside
// quotes or parentheses.
func splitPipeline(inner string) []string {
	var (
		stages []string
		start  int
		depth  int
		quote  byte
	)

	for i := 0; i < len(inner); i++ {
		c := inner[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case c == '|' && depth == 0:
			stages = append(stages, strings.TrimSpace(inner[start:i]))
			start = i + 1
		}
	}

	return append(stages, strings.TrimSpace(inner[start:]))
}

// firstToken returns the first whitespace-separated token of s.
func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexAny(s, " \t"); idx >= 0 {
		return s[:idx]
	}

	return s
}

// analyzeContext inspects the text around an action to decide whether the action is
// the entire YAML value (standalone) and, if so, whether it is wrapped in matching
// quotes. Non-standalone actions (embedded in a larger scalar, flow collections, …)
// return standalone=false and are skipped to avoid false positives.
func analyzeContext(pre, post string) (bool, bool) {
	post = strings.TrimRight(stripTrailingComment(post), " \t")

	var closeQuote byte

	if rest := strings.TrimLeft(post, " \t"); rest != "" {
		if rest[0] != '"' && rest[0] != '\'' {
			return false, false
		}

		closeQuote = rest[0]

		if strings.TrimSpace(rest[1:]) != "" {
			return false, false
		}
	}

	m := standaloneOpenRe.FindStringSubmatch(pre)
	if m == nil {
		return false, false
	}

	var openQuote byte
	if m[3] != "" {
		openQuote = m[3][0]
	}

	switch {
	case openQuote == 0 && closeQuote == 0:
		return true, false
	case openQuote != 0 && openQuote == closeQuote:
		return true, true
	default:
		// A quote on only one side is ambiguous; skip conservatively.
		return false, false
	}
}

// stripTrailingComment removes a YAML inline comment (a `#` preceded by whitespace or
// at the start) from s.
func stripTrailingComment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return s[:i]
		}
	}

	return s
}

// computeBlockScalarLines marks, for each line, whether it is inside a YAML
// block-scalar (literal/folded) body, where substitutions are part of a string and
// need no quoting.
func computeBlockScalarLines(lines []string) []bool {
	res := make([]bool, len(lines))

	inBlock := false
	blockIndent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if inBlock {
			if trimmed == "" || indent > blockIndent {
				res[i] = true
				continue
			}

			inBlock = false
		}

		if trimmed != "" && blockScalarHeaderRe.MatchString(line) {
			inBlock = true
			blockIndent = indent
		}
	}

	return res
}

// schemaWalker traverses an OpenAPI value schema and records the paths whose type
// resolves to an unvalidated string.
type schemaWalker struct {
	root map[string]any
	out  *riskyPaths
}

func (w *schemaWalker) walk(path string, schema map[string]any, depth int) {
	if schema == nil || depth > 64 {
		return
	}

	schema = w.resolveRef(schema)

	// Composition: walk every branch at the same path. A branch that is itself an
	// unvalidated string registers the path (union for oneOf/anyOf; for allOf the
	// merged validation keywords are evaluated per-branch, which is conservative).
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if subs, ok := schema[key].([]any); ok {
			for _, sub := range subs {
				if sm, ok := sub.(map[string]any); ok {
					w.walk(path, sm, depth+1)
				}
			}
		}
	}

	types := schemaTypes(schema)

	if props, ok := schema["properties"].(map[string]any); ok {
		for name, raw := range props {
			if ps, ok := raw.(map[string]any); ok {
				w.walk(joinPath(path, name), ps, depth+1)
			}
		}
	}

	if types["array"] && path != "" {
		if items, ok := schema["items"].(map[string]any); ok {
			resolved := w.resolveRef(items)
			if schemaTypes(resolved)["string"] && !hasValidation(resolved) {
				w.out.array[path] = struct{}{}
			}
		}
	}

	if types["string"] && path != "" && !hasValidation(schema) {
		w.out.scalar[path] = struct{}{}
	}
}

// resolveRef resolves a local `$ref` ("#/...") chain, merging sibling keys of the
// referring schema over the target. A non-local or unresolvable ref is returned as-is.
func (w *schemaWalker) resolveRef(schema map[string]any) map[string]any {
	for range 16 {
		ref, ok := schema["$ref"].(string)
		if !ok {
			return schema
		}

		if !strings.HasPrefix(ref, "#/") {
			return schema
		}

		target := w.root
		resolved := true

		for part := range strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/") {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")

			next, ok := target[part].(map[string]any)
			if !ok {
				resolved = false

				break
			}

			target = next
		}

		if !resolved {
			return schema
		}

		merged := make(map[string]any, len(target)+len(schema))
		maps.Copy(merged, target)

		for k, v := range schema {
			if k != "$ref" {
				merged[k] = v
			}
		}

		schema = merged
	}

	return schema
}

// schemaTypes returns the set of declared JSON-schema types (handles both a plain
// `type: string` and the nullable `type: [string, "null"]` form).
func schemaTypes(schema map[string]any) map[string]bool {
	res := make(map[string]bool)

	switch t := schema["type"].(type) {
	case string:
		res[t] = true
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok {
				res[s] = true
			}
		}
	}

	return res
}

// hasValidation reports whether the schema constrains a string enough that it never
// needs quoting: a pattern, an enum, or a format.
func hasValidation(schema map[string]any) bool {
	if s, ok := schema["pattern"].(string); ok && strings.TrimSpace(s) != "" {
		return true
	}

	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		return true
	}

	if f, ok := schema["format"].(string); ok && strings.TrimSpace(f) != "" {
		return true
	}

	return false
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}

	return base + "." + key
}
