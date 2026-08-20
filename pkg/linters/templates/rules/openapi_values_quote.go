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
	"toYaml": {}, "mustToYaml": {},
	"b64enc": {}, "b32enc": {},
	"sha1sum": {}, "sha256sum": {}, "sha512sum": {}, "adler32sum": {},
}

// blockOrSinkFuncs are pipeline functions after which quoting is impossible or
// meaningless, so a value routed through one of them is exempt from this rule:
//   - nindent/indent place the value as an indented (multi-line) block — e.g. a CA
//     bundle under a `key: |` block scalar, or a base64 blob expanded via `b64dec`;
//     wrapping such output in quotes would corrupt it, so "must be quoted" never applies.
//   - fail consumes the value into an aborting error message that is never rendered
//     into the manifest, so it cannot be a YAML-injection vector.
var blockOrSinkFuncs = map[string]struct{}{
	"nindent": {}, "indent": {}, "fail": {},
}

// templateActionRe matches a single Go-template action {{ ... }}, capturing the
// inner expression with the surrounding whitespace/`-` trim markers stripped.
var templateActionRe = regexp.MustCompile(`{{-?\s*(.*?)\s*-?}}`)

// blockScalarHeaderRe matches a YAML block-scalar header line (`key: |`, `- >-`,
// `key: |+2 # x`, …). Content indented under such a header is a literal string, so
// substitutions there are already part of a string and must not be quoted.
var blockScalarHeaderRe = regexp.MustCompile(`^(\s*)(-\s+)?([\w.\-/]+\s*:\s*)?[|>][+-]?\d*\s*(#.*)?$`)

// assignRe matches a template variable assignment `$name := expr` / `$name = expr`.
var assignRe = regexp.MustCompile(`^(\$[A-Za-z_]\w*)\s*(:?=)\s*(.+)$`)

// includeRe matches an `include "name" arg` / `template "name" arg` action, capturing
// the called template name and the argument expression.
var includeRe = regexp.MustCompile(`^(?:include|template)\s+"([^"]+)"\s*(.*)$`)

// defineNameRe captures the name of a `define "name"` block.
var defineNameRe = regexp.MustCompile(`^define\s+"([^"]+)"`)

// valuesRefRe finds a `.Values.<...>` / `$.Values.<...>` reference token, used to spot
// a risky value passed as a function argument (`{{ printf "%s" .Values.mod.foo }}`).
var valuesRefRe = regexp.MustCompile(`\$?\.Values\.[A-Za-z0-9_.]+`)

// varTokenRe finds a variable or dot-relative reference token (`$v`, `$s.host`,
// `.field`), used to spot a risky variable passed as a function argument.
var varTokenRe = regexp.MustCompile(`\$[A-Za-z_]\w*(?:\.[\w.]+)?|\.[A-Za-z_][\w.]*`)

// passthroughFuncs are functions whose output is (or contains) their string argument
// unchanged as far as YAML safety goes — so a risky value passed to one still needs
// quoting. The set is deliberately curated; an unknown leading function is left alone
// to avoid false positives.
var passthroughFuncs = map[string]struct{}{
	"printf": {}, "print": {}, "println": {},
	"default": {}, "coalesce": {}, "cat": {}, "toString": {},
	"upper": {}, "lower": {}, "title": {}, "untitle": {}, "nospace": {},
	"trim": {}, "trimAll": {}, "trimPrefix": {}, "trimSuffix": {},
	"replace": {}, "repeat": {}, "trunc": {}, "substr": {}, "abbrev": {},
	"snakecase": {}, "camelcase": {}, "kebabcase": {}, "swapcase": {},
}

// elementFuncs return an element (or the value) of their array/map argument, so a risky
// array/map argument yields a risky string result: `{{ index .Values.mod.list 0 }}`.
var elementFuncs = map[string]struct{}{
	"index": {}, "first": {}, "last": {}, "mustFirst": {}, "mustLast": {},
}

// valuePreRe splits the text before an action into the YAML plumbing (indentation, an
// optional list dash, an optional `key:` — a literal key or a template-action key such
// as `{{ $k }}:` from a map range) and the value text that precedes the action
// (captured group 3), which is empty for a standalone value and non-empty when the
// action is embedded in a larger scalar.
var valuePreRe = regexp.MustCompile(`^\s*(-\s+)?((?:\{\{[^{}]*\}\}|[\w.\-/]+)\s*:\s*)?(.*)$`)

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

// riskyPaths holds the value paths (relative to a schema level) whose type resolves to
// an unvalidated string. scalar paths are string leaves; array paths are arrays whose
// items are unvalidated strings (checked through `range`); object maps an
// array-of-objects path to the recursive risk profile of its element (checked through
// `range $x := …` and `{{ $x.field }}`, including a nested `range` over a string-array
// sub-field). The structure is recursive so any depth of range-in-range is covered.
type riskyPaths struct {
	scalar     map[string]struct{}
	array      map[string]struct{}
	object     map[string]*riskyPaths
	arrayArray map[string]struct{}
}

func newRiskyPaths() *riskyPaths {
	return &riskyPaths{
		scalar:     make(map[string]struct{}),
		array:      make(map[string]struct{}),
		object:     make(map[string]*riskyPaths),
		arrayArray: make(map[string]struct{}),
	}
}

func (p *riskyPaths) empty() bool {
	return len(p.scalar) == 0 && len(p.array) == 0 && len(p.object) == 0 && len(p.arrayArray) == 0
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

	type templateFile struct {
		relPath string
		lines   []string
	}

	parsed := make([]templateFile, 0, len(files))
	contents := make([]string, 0, len(files))

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		parsed = append(parsed, templateFile{fsutils.Rel(modulePath, filePath), strings.Split(string(content), "\n")})
		contents = append(contents, string(content))
	}

	// Cross-template flow: what each module `define` renders from its parameter, so a
	// risky value passed to it via include/template can be checked at the call site.
	defineEmits := resolveDefineEmits(collectDefines(contents))

	for _, f := range parsed {
		inBlock := computeBlockScalarLines(f.lines)

		r.checkScalars(f.relPath, f.lines, inBlock, valuesKey, risky, errorList)
		r.checkScopedEmissions(f.relPath, f.lines, inBlock, valuesKey, risky, defineEmits, errorList)
	}
}

// checkScalars reports unquoted standalone emissions of scalar string values.
func (r *OpenAPIValuesQuoteRule) checkScalars(
	relPath string, lines []string, inBlock []bool, valuesKey string,
	risky *riskyPaths, errorList *errors.LintRuleErrorsList,
) {
	for i, line := range lines {
		if inBlock[i] {
			continue
		}

		for _, loc := range templateActionRe.FindAllStringSubmatchIndex(line, -1) {
			inner := strings.TrimSpace(line[loc[2]:loc[3]])

			valPath, ok := scalarEmission(inner, valuesKey, risky)
			if !ok || !r.Enabled(valPath) {
				continue
			}

			if pipelineIsSafe(inner) {
				continue
			}

			report, wrap := analyzeContext(line[:loc[0]], line[loc[1]:])
			if !report {
				continue
			}

			errorList.WithFilePath(relPath).WithLineNumber(i+1).
				Errorf("value '.Values.%s.%s' is an OpenAPI string without a validation pattern (pattern/enum/format) %s",
					valuesKey, valPath, quoteAdvice(wrap))
		}
	}
}

// quoteAdvice tailors the fix hint: an embedded value must be wrapped as a whole,
// while a standalone value can also be piped through quote.
func quoteAdvice(wrap bool) string {
	if wrap {
		return "and must be quoted in templates: wrap the whole value in quotes"
	}

	return "and must be quoted in templates: use '| quote' or wrap the value in quotes"
}

// scalarEmission returns the risky scalar value path a direct emission
// (`{{ .Values.mod.foo }}`, or the parenthesised `{{ (.Values.mod.foo) }}`) renders,
// and whether it matched. Passthrough-function forms are handled in checkScopedEmissions
// (which also sees variables).
func scalarEmission(inner, valuesKey string, risky *riskyPaths) (string, bool) {
	if p, ok := valuePath(leadingCommand(inner), valuesKey); ok {
		return p, has(risky.scalar, p)
	}

	return "", false
}

// passthroughRisky returns the risky scalar value path passed as an argument to a
// passthrough function (`{{ printf "%s" .Values.mod.foo }}`, `{{ upper $v }}`), whether
// via a `.Values` reference or a variable, and whether it matched.
func passthroughRisky(inner, valuesKey string, risky *riskyPaths, innermostDot func() *loopScope, findVar func(string) *loopScope) (string, bool) {
	lead := leadingCommand(inner)
	if lead == "printf" && strings.Contains(inner, "%q") {
		return "", false // %q already quotes its output
	}

	// index/first/last yield an element of their array/map argument.
	if _, isElem := elementFuncs[lead]; isElem {
		for _, tok := range valuesRefRe.FindAllString(inner, -1) {
			if p, ok := valuePath(tok, valuesKey); ok && has(risky.array, p) {
				return p, true
			}
		}

		return "", false
	}

	for _, tok := range valuesRefRe.FindAllString(inner, -1) {
		if p, ok := valuePath(tok, valuesKey); ok && has(risky.scalar, p) {
			return p, true
		}
	}

	for _, tok := range varTokenRe.FindAllString(inner, -1) {
		if scope, sub := emissionScope(tok, innermostDot, findVar); scope != nil {
			return emissionPath(scope, sub), true
		}
	}

	return "", false
}

// isPassthrough reports whether a leading function forwards a risky argument to its
// output (a string transform, or an array/map element accessor).
func isPassthrough(lead string) bool {
	return has(passthroughFuncs, lead) || has(elementFuncs, lead)
}

// emissionPath renders the display value path for a tracked emission.
func emissionPath(scope *loopScope, sub string) string {
	if scope.kind != "object" {
		return scope.valPath
	}

	sep := "."
	if scope.elemArray {
		sep = "[]."
	}

	return scope.valPath + sep + sub
}

// loopScope tracks a template block while scanning for `range`/`with` over risky
// string values. When the block selects a risky value, kind is "array" (string-array
// element), "strarray" (an element that is itself a string array, for range-in-range),
// "object" (an object whose profile holds its risky sub-fields), or "scalar" (a
// `with`-scoped string); elemVar is the element accessor ("$x", or "." for
// dot-binding). kind is empty for any other block. valPath is the display path used in
// findings. elemArray marks an object whose sub-fields belong to array elements
// (`servers[].host`) rather than to a single object (`db.host`).
type loopScope struct {
	rebindDot bool
	kind      string
	elemVar   string
	valPath   string
	elemArray bool
	profile   *riskyPaths
}

// checkScopedEmissions reports unquoted emissions of a risky string reached through a
// `range` over a string array (`{{ range $e := .Values.mod.list }}{{ $e }}`) or a
// `with` over a string scalar (`{{ with .Values.mod.foo }}{{ . }}`), including the
// root-scoped `$.Values....` form used inside such blocks.
func (r *OpenAPIValuesQuoteRule) checkScopedEmissions(
	relPath string, lines []string, inBlock []bool, valuesKey string,
	risky *riskyPaths, defineEmits map[string]defineEmit, errorList *errors.LintRuleErrorsList,
) {
	var stack []loopScope

	assigned := map[string]loopScope{}

	innermostDotScope := func() *loopScope { return lookupAccessor(".", stack, assigned) }
	findVarScope := func(v string) *loopScope { return lookupAccessor(v, stack, assigned) }

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
				stack = append(stack, loopScope{rebindDot: false})
				continue
			case "define", "block":
				stack = append(stack, loopScope{rebindDot: true})
				continue
			case "with":
				stack = append(stack, resolveScope("with", inner, valuesKey, risky, stack, assigned, r))
				continue
			case "range":
				stack = append(stack, resolveScope("range", inner, valuesKey, risky, stack, assigned, r))
				continue
			}

			// Variable assignment (`{{ $y := … }}`) produces no output; it binds $y to
			// the risk of its right-hand side so later `{{ $y }}` / `{{ $y.field }}` is
			// still checked (aliasing).
			if name, rhs, ok := parseAssignment(inner); ok {
				if desc, ok := resolveAssigned(name, rhs, valuesKey, risky, stack, assigned, r); ok {
					assigned[name] = desc
				} else {
					delete(assigned, name)
				}

				continue
			}

			// Cross-template flow: a risky value passed to a module template that renders
			// it unquoted (`{{ include "mymod.env" .Values.mod.config }}`).
			if name, arg, ok := parseInclude(inner); ok {
				r.checkInclude(relPath, i+1, inner, name, arg, valuesKey, risky, stack, assigned, defineEmits, errorList)
				continue
			}

			// Passthrough function with a risky argument (`{{ printf "%s" .Values.mod.foo }}`,
			// `{{ upper $v }}`, `{{ index .Values.mod.list 0 }}`) — the risky value flows to
			// the unquoted output.
			if lead := leadingCommand(inner); isPassthrough(lead) {
				if path, ok := passthroughRisky(inner, valuesKey, risky, innermostDotScope, findVarScope); ok && !inBlock[i] && !pipelineIsSafe(inner) && r.Enabled(path) {
					if report, wrap := analyzeContext(line[:loc[0]], line[loc[1]:]); report {
						errorList.WithFilePath(relPath).WithLineNumber(i+1).
							Errorf("value '.Values.%s.%s' is an OpenAPI string without a validation pattern (pattern/enum/format) %s",
								valuesKey, path, quoteAdvice(wrap))
					}
				}

				continue
			}

			tracked, sub := emissionScope(leadingCommand(inner), innermostDotScope, findVarScope)
			if tracked == nil {
				continue
			}

			if inBlock[i] || pipelineIsSafe(inner) {
				continue
			}

			report, wrap := analyzeContext(line[:loc[0]], line[loc[1]:])
			if !report {
				continue
			}

			r.reportScoped(relPath, i+1, valuesKey, tracked, sub, wrap, errorList)
		}
	}
}

// reportScoped emits the finding for a scoped emission, worded for a scalar value, an
// array element, or a string sub-field of an array-of-objects element. The full value
// path (e.g. `servers[].host`) is checked against the exclude list so a specific
// sub-field can be excluded, not only its enclosing array.
func (r *OpenAPIValuesQuoteRule) reportScoped(
	relPath string, line int, valuesKey string, scope *loopScope, sub string, wrap bool, errorList *errors.LintRuleErrorsList,
) {
	path := emissionPath(scope, sub)
	if !r.Enabled(path) {
		return
	}

	tail := "is an OpenAPI string without a validation pattern (pattern/enum/format) " + quoteAdvice(wrap)

	el := errorList.WithFilePath(relPath).WithLineNumber(line)

	switch {
	case scope.kind == "array":
		el.Errorf("array element from '.Values.%s.%s' %s", valuesKey, path, tail)
	case scope.kind == "object" && scope.elemArray:
		el.Errorf("array element field '.Values.%s.%s' %s", valuesKey, path, tail)
	default:
		el.Errorf("value '.Values.%s.%s' %s", valuesKey, path, tail)
	}
}

// resolveScope builds the scope for a `range` or `with` block. It resolves the block
// subject to the risky namespace it selects — either root-scoped (`.Values.<key>.<p>` /
// `$.Values.<key>.<p>`) or relative to an enclosing element (`$s.<sub>` / `.<sub>`, the
// key to range-in-range) — and classifies it. A `range` tracks a risky string array
// (kind "array") or array-of-objects (kind "object"); a `with` tracks a risky string
// scalar (kind "scalar"). Any other block returns an untracked, dot-rebinding scope.
func resolveScope(block, inner, valuesKey string, root *riskyPaths, stack []loopScope, assigned map[string]loopScope, rule *OpenAPIValuesQuoteRule) loopScope {
	sc := loopScope{rebindDot: true}

	var subject, elemVar string

	if block == "range" {
		coll, ev := parseRange(inner)
		subject, elemVar = firstToken(coll), ev
	} else {
		subject = firstToken(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(inner), "with")))
		elemVar = "."
	}

	// A bare variable / dot subject (`range $x`, `with $s`, `range $c := $row`) resolves
	// against the enclosing scope or an assigned variable rather than a `.Values` path.
	if accessor, tail := splitVarPath(subject); tail == "" && accessor != "" {
		v := lookupAccessor(accessor, stack, assigned)
		if v == nil || v.kind == "" {
			return sc
		}

		if out, ok := scopeFromVar(block, v, elemVar); ok && rule.Enabled(out.valPath) {
			return out
		}

		return sc
	}

	base, sub, disp, ok := resolveBase(subject, valuesKey, root, stack, assigned)
	if !ok || !rule.Enabled(disp) {
		return sc
	}

	switch {
	case block == "range" && has(base.array, sub):
		sc.kind = "array"
	case block == "range" && has(base.arrayArray, sub):
		sc.kind = "strarray"
	case block == "range" && has(base.object, sub):
		sc.kind = "object"
		sc.elemArray = true
		sc.profile = base.object[sub]
	case block == "with" && has(base.scalar, sub):
		sc.kind = "scalar"
	case block == "with":
		// `with` over an object value binds dot to the object; its risky string
		// sub-fields are then reachable as `{{ .field }}` (a single object, so
		// sub-fields display as `db.host`, not `db[].host`).
		prof := subProfile(base, sub)
		if prof == nil {
			return sc
		}

		sc.kind = "object"
		sc.profile = prof
	default:
		return sc
	}

	sc.elemVar = elemVar
	sc.valPath = disp

	return sc
}

// subProfile returns the risk profile of the object at prefix within base, with keys
// re-based to be relative to it, or nil when nothing risky lives under it. It lets a
// `with` over a (non-array) object reuse the same sub-field machinery as an
// array-of-objects element.
func subProfile(base *riskyPaths, prefix string) *riskyPaths {
	pfx := prefix + "."
	out := newRiskyPaths()

	for k := range base.scalar {
		if rel, ok := strings.CutPrefix(k, pfx); ok {
			out.scalar[rel] = struct{}{}
		}
	}

	for k := range base.array {
		if rel, ok := strings.CutPrefix(k, pfx); ok {
			out.array[rel] = struct{}{}
		}
	}

	for k := range base.arrayArray {
		if rel, ok := strings.CutPrefix(k, pfx); ok {
			out.arrayArray[rel] = struct{}{}
		}
	}

	for k, v := range base.object {
		if rel, ok := strings.CutPrefix(k, pfx); ok {
			out.object[rel] = v
		}
	}

	if out.empty() {
		return nil
	}

	return out
}

// scopeFromVar builds a range/with scope from a variable's tracked risk. A `with`
// tracks an object variable (`{{ with $s }}{{ .host }}`); a `range` tracks a variable
// that holds — or whose element is — a string array (array-of-arrays inner range,
// `{{ range $x := .Values.list }}` bound then `{{ range $x }}`) or an array of objects.
// The bool is false when the variable is not usefully rangeable/scopeable.
func scopeFromVar(block string, v *loopScope, elemVar string) (loopScope, bool) {
	sc := loopScope{rebindDot: true}

	if block == "with" {
		if v.kind == "object" {
			sc.kind, sc.elemVar, sc.valPath, sc.elemArray, sc.profile = "object", ".", v.valPath, v.elemArray, v.profile
			return sc, true
		}

		return sc, false
	}

	switch v.kind {
	case "strarray":
		sc.kind, sc.elemVar, sc.valPath = "array", elemVar, v.valPath+"[]"
	case "arrayvalue":
		sc.kind, sc.elemVar, sc.valPath = "array", elemVar, v.valPath
	case "objarrayvalue":
		sc.kind, sc.elemVar, sc.valPath, sc.elemArray, sc.profile = "object", elemVar, v.valPath, true, v.profile
	default:
		return sc, false
	}

	return sc, true
}

// resolveBase maps a block subject command to the risk namespace it lives in, the
// sub-key within that namespace, and the display path. Root references resolve against
// the module values root; `$var.<sub>` / `.<sub>` resolve against the risk profile of
// the enclosing array-of-objects element (enabling range-in-range).
func resolveBase(cmd, valuesKey string, root *riskyPaths, stack []loopScope, assigned map[string]loopScope) (*riskyPaths, string, string, bool) {
	if p, ok := valuePath(cmd, valuesKey); ok {
		return root, p, p, true
	}

	accessor, sub := splitVarPath(cmd)
	if accessor == "" || sub == "" {
		return nil, "", "", false
	}

	sc := lookupAccessor(accessor, stack, assigned)
	if sc == nil || sc.kind != "object" || sc.profile == nil {
		return nil, "", "", false
	}

	sep := "."
	if sc.elemArray {
		sep = "[]."
	}

	return sc.profile, sub, sc.valPath + sep + sub, true
}

// lookupAccessor finds the scope an element accessor refers to: a bare dot resolves to
// the innermost dot-rebinding scope (which may be untracked — the caller checks), a
// `$var` to the nearest enclosing range/with element variable or, failing that, a
// variable bound by an assignment (`{{ $y := … }}`).
func lookupAccessor(accessor string, stack []loopScope, assigned map[string]loopScope) *loopScope {
	for i := len(stack) - 1; i >= 0; i-- {
		if accessor == "." && stack[i].rebindDot {
			return &stack[i]
		}

		if accessor != "." && stack[i].elemVar == accessor {
			return &stack[i]
		}
	}

	if accessor != "." {
		if sc, ok := assigned[accessor]; ok {
			return &sc
		}
	}

	return nil
}

// has reports whether key k is present in map m of any value type.
func has[V any](m map[string]V, k string) bool {
	_, ok := m[k]
	return ok
}

// parseAssignment recognizes a template variable assignment `$name := expr` /
// `$name = expr` and returns the variable name (with `$`) and the right-hand side.
func parseAssignment(inner string) (string, string, bool) {
	m := assignRe.FindStringSubmatch(strings.TrimSpace(inner))
	if m == nil {
		return "", "", false
	}

	rhs := strings.TrimSpace(m[3])
	if strings.HasPrefix(rhs, "=") { // `==` comparison, not an assignment
		return "", "", false
	}

	return m[1], rhs, true
}

// resolveValueExpr resolves an expression to the risk of the VALUE it denotes: an
// alias of a risky variable (`$x`), a risky scalar (`.Values.mod.foo` / `$s.host`), or
// a risky object whose profile is returned (`.Values.mod.db`, an array-of-objects
// element, …). It is the shared basis for variable assignment and `include`/`template`
// argument analysis. The returned scope has no elemVar; the caller sets one if needed.
func resolveValueExpr(cmd, valuesKey string, root *riskyPaths, stack []loopScope, assigned map[string]loopScope) (loopScope, bool) {
	if accessor, tail := splitVarPath(cmd); tail == "" && (accessor == "." || strings.HasPrefix(accessor, "$")) {
		src := lookupAccessor(accessor, stack, assigned)
		if src == nil || src.kind == "" {
			return loopScope{}, false
		}

		alias := *src
		alias.elemVar = ""
		alias.rebindDot = false

		return alias, true
	}

	base, sub, disp, ok := resolveBase(cmd, valuesKey, root, stack, assigned)
	if !ok {
		return loopScope{}, false
	}

	switch {
	case has(base.scalar, sub):
		return loopScope{kind: "scalar", valPath: disp}, true
	case has(base.array, sub):
		return loopScope{kind: "arrayvalue", valPath: disp}, true
	case has(base.object, sub):
		return loopScope{kind: "objarrayvalue", valPath: disp, elemArray: true, profile: base.object[sub]}, true
	}

	if prof := subProfile(base, sub); prof != nil {
		return loopScope{kind: "object", valPath: disp, profile: prof}, true
	}

	return loopScope{}, false
}

// resolveAssigned computes the risk a variable takes on from an assignment's
// right-hand side. It returns ok=false when the right-hand side is not risky (or
// already made safe by a pipeline function), so the caller clears any previous binding.
func resolveAssigned(name, rhs, valuesKey string, root *riskyPaths, stack []loopScope, assigned map[string]loopScope, rule *OpenAPIValuesQuoteRule) (loopScope, bool) {
	if pipelineIsSafe(rhs) {
		return loopScope{}, false
	}

	desc, ok := resolveValueExpr(leadingCommand(rhs), valuesKey, root, stack, assigned)
	if !ok || !rule.Enabled(desc.valPath) {
		return loopScope{}, false
	}

	desc.elemVar = name

	return desc, true
}

// defineInfo records what a `define` block renders from its parameter (dot): whether it
// emits the bare parameter unquoted, which parameter-relative scalar sub-fields it emits
// unquoted, which parameter-relative array sub-fields it ranges and emits elements of
// unquoted, and which templates it forwards its parameter (or a sub-field) to.
type defineInfo struct {
	bareDot   bool
	subs      map[string]struct{}
	arraySubs map[string]struct{}
	calls     []defineCall
}

type defineCall struct {
	name string
	sub  string // "" = forwarded `.`, "a.b" = forwarded `.a.b`
}

// defineEmit is the resolved (transitive) set of parameter-relative values a template
// renders unquoted.
type defineEmit struct {
	bareDot   bool
	subs      map[string]struct{}
	arraySubs map[string]struct{}
}

// defineFrame is one entry on the block stack while scanning a define body.
type defineFrame struct {
	def      *defineInfo
	rebind   bool
	rangeSub string // set when this range iterates the parameter sub-field <rangeSub>
	elemVar  string // the range element accessor ("." or "$x")
}

// parseInclude recognizes an `include "name" arg` / `template "name" arg` action and
// returns the template name and the (possibly empty) argument expression.
func parseInclude(inner string) (string, string, bool) {
	m := includeRe.FindStringSubmatch(strings.TrimSpace(inner))
	if m == nil {
		return "", "", false
	}

	return m[1], strings.TrimSpace(m[2]), true
}

// collectDefines scans the given template contents for `define` blocks and records, per
// template name, the parameter-relative values it renders unquoted (see defineInfo).
// Only the module's own defines are seen; external ones (helm_lib, …) are absent and
// therefore never matched, which keeps cross-template findings free of false positives.
func collectDefines(contents []string) map[string]*defineInfo {
	defs := map[string]*defineInfo{}

	for _, content := range contents {
		lines := strings.Split(content, "\n")
		inBlock := computeBlockScalarLines(lines)

		var stack []defineFrame

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
				case "define":
					stack = append(stack, defineFrame{def: defForName(defs, inner)})
					continue
				case "if":
					stack = append(stack, defineFrame{})
					continue
				case "with", "block":
					stack = append(stack, defineFrame{rebind: true})
					continue
				case "range":
					stack = append(stack, rangeDefineFrame(stack, inner))
					continue
				}

				def, depth, innermost := defineContext(stack)
				if def == nil || inBlock[i] {
					continue
				}

				switch {
				case depth == 0:
					recordDefineUsage(def, inner, line[:loc[0]], line[loc[1]:])
				case depth == 1 && innermost != nil && innermost.rangeSub != "":
					recordDefineArrayElem(def, innermost, inner, line[:loc[0]], line[loc[1]:])
				}
			}
		}
	}

	return defs
}

// defineContext returns the innermost enclosing define, the dot-rebinding depth within
// it, and the innermost rebinding frame (for spotting parameter-subfield ranges).
func defineContext(stack []defineFrame) (*defineInfo, int, *defineFrame) {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].def == nil {
			continue
		}

		depth := 0

		var innermost *defineFrame

		for j := i + 1; j < len(stack); j++ {
			if stack[j].rebind {
				depth++
				innermost = &stack[j]
			}
		}

		return stack[i].def, depth, innermost
	}

	return nil, 0, nil
}

// rangeDefineFrame builds the stack frame for a `range` inside a define, marking it when
// it iterates a parameter sub-field at the parameter level (`{{ range .items }}`).
func rangeDefineFrame(stack []defineFrame, inner string) defineFrame {
	f := defineFrame{rebind: true}

	if def, depth, _ := defineContext(stack); def == nil || depth != 0 {
		return f
	}

	coll, elemVar := parseRange(inner)
	if accessor, sub := splitVarPath(firstToken(coll)); accessor == "." && sub != "" {
		f.rangeSub = sub
		f.elemVar = elemVar
	}

	return f
}

// defForName returns (creating if needed) the defineInfo for the `define "name"`
// action, or nil when the name cannot be parsed.
func defForName(defs map[string]*defineInfo, inner string) *defineInfo {
	m := defineNameRe.FindStringSubmatch(inner)
	if m == nil {
		return nil
	}

	if info := defs[m[1]]; info != nil {
		return info
	}

	info := &defineInfo{subs: map[string]struct{}{}, arraySubs: map[string]struct{}{}}
	defs[m[1]] = info

	return info
}

// recordDefineUsage records a parameter-relative emission or a parameter-forwarding
// include for the enclosing define (at the parameter level).
func recordDefineUsage(def *defineInfo, inner, pre, post string) {
	if pipelineIsSafe(inner) {
		return
	}

	if name, arg, ok := parseInclude(inner); ok {
		if accessor, sub := splitVarPath(leadingCommand(arg)); accessor == "." {
			def.calls = append(def.calls, defineCall{name: name, sub: sub})
		}

		return
	}

	accessor, sub := splitVarPath(leadingCommand(inner))
	if accessor != "." {
		return
	}

	if report, _ := analyzeContext(pre, post); !report {
		return
	}

	if sub == "" {
		def.bareDot = true
	} else {
		def.subs[sub] = struct{}{}
	}
}

// recordDefineArrayElem records that a define renders, unquoted, the elements of the
// parameter array sub-field its enclosing `range` iterates.
func recordDefineArrayElem(def *defineInfo, rf *defineFrame, inner, pre, post string) {
	if pipelineIsSafe(inner) {
		return
	}

	accessor, sub := splitVarPath(leadingCommand(inner))
	if sub != "" || accessor != rf.elemVar {
		return
	}

	if report, _ := analyzeContext(pre, post); !report {
		return
	}

	def.arraySubs[rf.rangeSub] = struct{}{}
}

// resolveDefineEmits resolves each define's own and transitively forwarded emissions
// into a flat parameter-relative emit set per template name.
func resolveDefineEmits(defs map[string]*defineInfo) map[string]defineEmit {
	out := make(map[string]defineEmit, len(defs))

	var resolve func(name string, seen map[string]bool) defineEmit

	resolve = func(name string, seen map[string]bool) defineEmit {
		info := defs[name]
		if info == nil || seen[name] {
			return defineEmit{subs: map[string]struct{}{}, arraySubs: map[string]struct{}{}}
		}

		seen[name] = true
		defer delete(seen, name)

		res := defineEmit{bareDot: info.bareDot, subs: map[string]struct{}{}, arraySubs: map[string]struct{}{}}
		for s := range info.subs {
			res.subs[s] = struct{}{}
		}

		for s := range info.arraySubs {
			res.arraySubs[s] = struct{}{}
		}

		for _, call := range info.calls {
			sub := resolve(call.name, seen)

			if call.sub == "" {
				res.bareDot = res.bareDot || sub.bareDot
				mergeSet(res.subs, sub.subs, "")
				mergeSet(res.arraySubs, sub.arraySubs, "")

				continue
			}

			if sub.bareDot {
				res.subs[call.sub] = struct{}{}
			}

			mergeSet(res.subs, sub.subs, call.sub+".")
			mergeSet(res.arraySubs, sub.arraySubs, call.sub+".")
		}

		return res
	}

	for name := range defs {
		out[name] = resolve(name, map[string]bool{})
	}

	return out
}

// mergeSet copies every key of src into dst, each prefixed by prefix.
func mergeSet(dst, src map[string]struct{}, prefix string) {
	for s := range src {
		dst[prefix+s] = struct{}{}
	}
}

// checkInclude flags a risky value that reaches a module template which renders it
// unquoted. It fires only when the argument resolves to a concrete risky value and the
// called template's body (seen in the module) emits it unquoted.
func (r *OpenAPIValuesQuoteRule) checkInclude(
	relPath string, line int, inner, name, arg, valuesKey string,
	root *riskyPaths, stack []loopScope, assigned map[string]loopScope,
	emits map[string]defineEmit, errorList *errors.LintRuleErrorsList,
) {
	emit, ok := emits[name]
	if !ok || pipelineHasSafeFunc(inner) {
		return
	}

	desc, ok := resolveValueExpr(leadingCommand(arg), valuesKey, root, stack, assigned)
	if !ok {
		return
	}

	report := func(path string) {
		if !r.Enabled(path) {
			return
		}

		errorList.WithFilePath(relPath).WithLineNumber(line).
			Errorf("value '.Values.%s.%s' is an OpenAPI string without a validation pattern (pattern/enum/format) and is rendered unquoted by template %q; quote it there or before passing to the template",
				valuesKey, path, name)
	}

	switch {
	case desc.kind == "scalar" && emit.bareDot:
		report(desc.valPath)
	case desc.kind == "object" && desc.profile != nil:
		sep := "."
		if desc.elemArray {
			sep = "[]."
		}

		for sub := range emit.subs {
			if has(desc.profile.scalar, sub) {
				report(desc.valPath + sep + sub)
			}
		}

		for sub := range emit.arraySubs {
			if has(desc.profile.array, sub) {
				report(desc.valPath + sep + sub)
			}
		}
	}
}

// emissionScope resolves an emission command to the tracked scope it belongs to and,
// for an array-of-objects element, the risky sub-field it accesses. It returns
// (nil, "") when the command is not a tracked risky emission.
//
// The command is split into the element accessor and an optional sub-path: `.`/`$x`
// (the element itself) or `.field`/`$x.field.sub` (a sub-field). A bare-dot accessor
// resolves to the innermost dot-rebinding scope; a `$var` accessor to the nearest
// enclosing scope that introduced it.
func emissionScope(cmd string, innermostDot func() *loopScope, findVar func(string) *loopScope) (*loopScope, string) {
	accessor, sub := splitVarPath(cmd)
	if accessor == "" {
		return nil, ""
	}

	var s *loopScope
	if accessor == "." {
		s = innermostDot()
	} else {
		s = findVar(accessor)
	}

	if s == nil || s.kind == "" || s.elemVar != accessor {
		return nil, ""
	}

	switch s.kind {
	case "array", "scalar":
		if sub == "" {
			return s, ""
		}
	case "object":
		if s.profile != nil && has(s.profile.scalar, sub) {
			return s, sub
		}
	}

	return nil, ""
}

// splitVarPath splits an emission command into the loop-element accessor and the
// sub-path accessed on it: "." -> (".", ""), ".host" -> (".", "host"),
// "$s" -> ("$s", ""), "$s.spec.name" -> ("$s", "spec.name"). Any other command
// (a function, a `.Values...` reference, …) yields ("", "").
func splitVarPath(cmd string) (string, string) {
	switch {
	case cmd == ".":
		return ".", ""
	case strings.HasPrefix(cmd, "$"):
		if accessor, sub, found := strings.Cut(cmd, "."); found {
			return accessor, sub
		}

		return cmd, ""
	case strings.HasPrefix(cmd, "."):
		return ".", cmd[1:]
	default:
		return "", ""
	}
}

// valuePath returns the module value path a command refers to via `.Values.<key>.<p>`
// or the root-scoped `$.Values.<key>.<p>` (valid inside range/with), and whether it
// matched.
func valuePath(cmd, valuesKey string) (string, bool) {
	for _, prefix := range []string{".Values." + valuesKey + ".", "$.Values." + valuesKey + "."} {
		if rest, ok := strings.CutPrefix(cmd, prefix); ok {
			return rest, true
		}
	}

	return "", false
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
// A first stage fully wrapped in parentheses is unwrapped first, so `(.Values.x)`
// reads as `.Values.x`.
func leadingCommand(inner string) string {
	stages := splitPipeline(inner)
	if len(stages) == 0 {
		return ""
	}

	s := strings.TrimSpace(stages[0])
	for isParenWrapped(s) {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}

	return firstToken(s)
}

// isParenWrapped reports whether s is a single parenthesised group, i.e. its first
// `(` matches its final `)`.
func isParenWrapped(s string) bool {
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return false
	}

	depth := 0

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(s)-1
			}
		}
	}

	return false
}

// pipelineHasSafeFunc reports whether any stage of the pipeline emits a YAML-safe
// scalar (see safeFuncs) — i.e. the value's own rendered output is quoted or encoded.
// This is the check to use when a value is emitted transitively (e.g. inside an
// included template), where only real quoting of the output — not block re-indenting —
// can protect it.
func pipelineHasSafeFunc(inner string) bool {
	for _, stage := range splitPipeline(inner)[1:] {
		if _, ok := safeFuncs[firstToken(stage)]; ok {
			return true
		}
	}

	return false
}

// pipelineIsSafe reports whether the value emitted directly by this action needs no
// explicit quote — either because a stage emits a YAML-safe scalar (pipelineHasSafeFunc)
// or because the value is placed as a block / never emitted (see blockOrSinkFuncs), in
// which case quoting is impossible or meaningless.
//
// blockOrSinkFuncs only exempts a value flowing directly into the pipeline. For an
// include/template argument the value is emitted inside the callee (see checkInclude),
// where an outer nindent/indent re-indents but does not quote it, so that path uses
// pipelineHasSafeFunc instead.
func pipelineIsSafe(inner string) bool {
	if pipelineHasSafeFunc(inner) {
		return true
	}

	for _, stage := range splitPipeline(inner)[1:] {
		if _, ok := blockOrSinkFuncs[firstToken(stage)]; ok {
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

// analyzeContext inspects the text around an action to decide whether the value must
// be reported and, if so, how. report is false when the value is wrapped in matching
// quotes (safe) or the context is ambiguous (a lone quote on one side). wrap is true
// when the action is embedded in a larger unquoted scalar (`host: pre-{{ … }}`), where
// the fix is to quote the whole value rather than pipe it through `quote`.
func analyzeContext(pre, post string) (bool, bool) {
	m := valuePreRe.FindStringSubmatch(pre)
	if m == nil {
		return false, false
	}

	valuePre := strings.TrimLeft(m[3], " \t")

	var openQuote byte
	if len(valuePre) > 0 && (valuePre[0] == '"' || valuePre[0] == '\'') {
		openQuote = valuePre[0]
		valuePre = valuePre[1:]
	}

	tail := strings.TrimRight(stripTrailingComment(post), " \t")

	var closeQuote byte
	if len(tail) > 0 && (tail[len(tail)-1] == '"' || tail[len(tail)-1] == '\'') {
		closeQuote = tail[len(tail)-1]
		tail = tail[:len(tail)-1]
	}

	embedded := strings.TrimSpace(valuePre) != "" || strings.TrimSpace(tail) != ""

	switch {
	case openQuote != 0 && openQuote == closeQuote:
		return false, false // wrapped in matching quotes -> safe
	case openQuote != 0 || closeQuote != 0:
		return false, false // a lone quote on one side -> ambiguous, skip
	case embedded:
		return true, true // embedded in a larger unquoted scalar -> wrap the whole value
	default:
		return true, false // standalone, unquoted -> pipe through quote
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
			w.walkItems(path, w.resolveRef(items), depth)
		}
	}

	// A map's values (additionalProperties) are iterated by `range` exactly like array
	// elements — the loop value is the map value — so they are classified the same way.
	if ap, ok := schema["additionalProperties"].(map[string]any); ok && path != "" {
		w.walkItems(path, w.resolveRef(ap), depth)
	}

	if types["string"] && path != "" && !hasValidation(schema) {
		w.out.scalar[path] = struct{}{}
	}
}

// walkItems classifies the element/value schema of an array or map: a plain
// unvalidated string registers the array path; an array of unvalidated strings
// registers an array-of-arrays path (checked through range-in-range); an object
// registers the element's recursive risk profile so `range $x := arr` /
// `{{ $x.field }}` (and nested ranges) can be checked.
func (w *schemaWalker) walkItems(path string, items map[string]any, depth int) {
	itemTypes := schemaTypes(items)

	if itemTypes["string"] && !hasValidation(items) {
		w.out.array[path] = struct{}{}
		return
	}

	if itemTypes["array"] {
		if inner, ok := items["items"].(map[string]any); ok {
			resolved := w.resolveRef(inner)
			if schemaTypes(resolved)["string"] && !hasValidation(resolved) {
				w.out.arrayArray[path] = struct{}{}
			}
		}

		return
	}

	if _, ok := items["properties"].(map[string]any); !ok {
		return
	}

	sub := newRiskyPaths()
	(&schemaWalker{root: w.root, out: sub}).walk("", items, depth+1)

	if !sub.empty() {
		w.out.object[path] = sub
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
