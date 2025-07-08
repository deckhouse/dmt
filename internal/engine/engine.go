/*
Copyright The Helm Authors.

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

package engine

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"errors"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/logger"
)

// Engine is an implementation of the Helm rendering implementation for templates.
type Engine struct {
	// If strict is enabled, template rendering will fail if a template references
	// a value that was not passed in.
	Strict bool
	// In LintMode, some 'required' template values may be missing, so don't fail
	LintMode bool
	// EnableDNS tells the engine to allow DNS lookups when rendering templates
	EnableDNS bool
}

// New creates a new instance of Engine using the passed in rest config.
func New() Engine {
	return Engine{}
}

// Render takes a chart, optional values, and value overrides, and attempts to render the Go templates.
//
// Render can be called repeatedly on the same engine.
//
// This will look in the chart's 'templates' data (e.g. the 'templates/' directory)
// and attempt to render the templates there using the values passed in.
//
// Values are scoped to their templates. A dependency template will not have
// access to the values set for its parent. If chart "foo" includes chart "bar",
// "bar" will not have access to the values for "foo".
//
// Values should be prepared with something like `chartutils.ReadValues`.
//
// Values are passed through the templates according to scope. If the top layer
// chart includes the chart foo, which includes the chart bar, the values map
// will be examined for a table called "foo". If "foo" is found in vals,
// that section of the values will be passed into the "foo" chart. And if that
// section contains a value named "bar", that value will be passed on to the
// bar chart during render time.
func (e Engine) Render(chrt *chart.Chart, values chartutil.Values) (map[string]string, error) {
	tmap := allTemplates(chrt, values)
	return e.render(tmap)
}

// Render takes a chart, optional values, and value overrides, and attempts to
// render the Go templates using the default options.
func Render(chrt *chart.Chart, values chartutil.Values) (map[string]string, error) {
	return new(Engine).Render(chrt, values)
}

// renderable is an object that can be rendered.
type renderable struct {
	// tpl is the current template.
	tpl string
	// vals are the values to be supplied to the template.
	vals chartutil.Values
	// namespace prefix to the templates of the current chart
	basePath string
}

const warnStartDelim = "HELM_ERR_START"
const warnEndDelim = "HELM_ERR_END"
const recursionMaxNums = 1000

var warnRegex = regexp.MustCompile(warnStartDelim + `((?s).*)` + warnEndDelim)

func warnWrap(warn string) string {
	return warnStartDelim + warn + warnEndDelim
}

// manageRecursion manages template recursion to prevent infinite loops
func manageRecursion(includedNames map[string]int, name string, maxRecursion int) error {
	if v, ok := includedNames[name]; ok {
		if v > maxRecursion {
			return fmt.Errorf("rendering template has a nested reference name: %s", name)
		}
		includedNames[name]++
	} else {
		includedNames[name] = 1
	}
	return nil
}

// decrementRecursion decrements the recursion counter for a template name
func decrementRecursion(includedNames map[string]int, name string) {
	if v, ok := includedNames[name]; ok && v > 0 {
		includedNames[name]--
	}
}

// 'include' needs to be defined in the scope of a 'tpl' template as
// well as regular file-loaded templates.
func includeFun(t *template.Template, includedNames map[string]int) func(string, any) (string, error) {
	return func(name string, data any) (string, error) {
		var buf strings.Builder

		if err := manageRecursion(includedNames, name, recursionMaxNums); err != nil {
			return "", err
		}
		defer decrementRecursion(includedNames, name)

		err := t.ExecuteTemplate(&buf, name, data)
		return buf.String(), err
	}
}

// As does 'tpl', so that nested calls to 'tpl' see the templates
// defined by their enclosing contexts.
func tplFun(parent *template.Template, includedNames map[string]int, strict bool) func(string, any) (string, error) {
	return func(tpl string, vals any) (string, error) {
		t, err := parent.Clone()
		if err != nil {
			return "", fmt.Errorf("cannot clone template: %w", err)
		}

		// Re-inject the missingkey option, see text/template issue https://github.com/golang/go/issues/43022
		// We have to go by strict from our engine configuration, as the option fields are private in Template.
		// TODO: Remove workaround (and the strict parameter) once we build only with golang versions with a fix.
		if strict {
			t.Option("missingkey=error")
		} else {
			t.Option("missingkey=zero")
		}

		// Re-inject 'include' so that it can close over our clone of t;
		// this lets any 'define's inside tpl be 'include'd.
		t.Funcs(template.FuncMap{
			"include": includeFun(t, includedNames),
			"tpl":     tplFun(t, includedNames, strict),
		})

		// We need a .New template, as template text which is just blanks
		// or comments after parsing out defines just adds new named
		// template definitions without changing the main template.
		// https://pkg.go.dev/text/template#Template.Parse
		// Use the parent's name for lack of a better way to identify the tpl
		// text string. (Maybe we could use a hash appended to the name?)
		t, err = t.New(parent.Name()).Parse(tpl)
		if err != nil {
			return "", fmt.Errorf("cannot parse template %q: %w", tpl, err)
		}

		var buf strings.Builder
		if err := t.Execute(&buf, vals); err != nil {
			return "", fmt.Errorf("error during tpl function execution for %q: %w", tpl, err)
		}

		// See comment in renderWithReferences explaining the <no value> hack.
		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}

// initFuncMap creates the Engine's FuncMap and adds context-specific functions.
func (e Engine) initFuncMap(t *template.Template) {
	funcMap := funcMap()
	includedNames := make(map[string]int)

	// Add the template-rendering functions here so we can close over t.
	funcMap["include"] = includeFun(t, includedNames)
	funcMap["tpl"] = tplFun(t, includedNames, e.Strict)

	// Add the `required` function here so we can use lintMode
	funcMap["required"] = func(warn string, val any) (any, error) {
		if val == nil {
			if e.LintMode {
				// Don't fail on missing required values when linting
				logger.WarnF("[WARNING] Missing required value: %s", warn)
				return "", nil
			}
			return val, errors.New(warnWrap(warn))
		} else if _, ok := val.(string); ok {
			if val == "" {
				if e.LintMode {
					// Don't fail on missing required values when linting
					logger.ErrorF("[ERROR] Missing required value: %s", warn)
					return "", nil
				}
				return val, errors.New(warnWrap(warn))
			}
		}
		return val, nil
	}

	// Override sprig fail function for linting and wrapping message
	funcMap["fail"] = func(msg string) (string, error) {
		if e.LintMode {
			// Don't fail when linting
			logger.WarnF("[WARNING] Fail: %s", msg)
			return "", nil
		}
		return "", errors.New(warnWrap(msg))
	}

	// When DNS lookups are not enabled override the sprig function and return
	// an empty string.
	if !e.EnableDNS {
		funcMap["getHostByName"] = func(_ string) string {
			return ""
		}
	}

	t.Funcs(funcMap)
}

// getRenderedContent returns the rendered content from the buffer with "<no value>" replaced
func getRenderedContent(buf *strings.Builder) string {
	if buf.Len() == 0 {
		return ""
	}
	return strings.ReplaceAll(buf.String(), "<no value>", "")
}

// render takes a map of templates/values and renders them.
//
//nolint:nonamedreturns // copy from helm
func (e Engine) render(tpls map[string]renderable) (rendered map[string]string, err error) {
	// Basically, what we do here is start with an empty parent template and then
	// build up a list of templates -- one for each file. Once all of the templates
	// have been parsed, we loop through again and execute every template.
	//
	// The idea with this process is to make it possible for more complex templates
	// to share common blocks, but to make the entire thing feel like a file-based
	// template engine.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("rendering template failed: %v", r)
		}
	}()
	t := template.New("gotpl")
	if e.Strict {
		t.Option("missingkey=error")
	} else {
		// Not that zero will attempt to add default values for types it knows,
		// but will still emit <no value> for others. We mitigate that later.
		t.Option("missingkey=zero")
	}

	e.initFuncMap(t)

	// We want to parse the templates in a predictable order. The order favors
	// higher-level (in file system) templates over deeply nested templates.
	keys := sortTemplates(tpls)

	for _, filename := range keys {
		r := tpls[filename]
		if _, err := t.New(filename).Parse(r.tpl); err != nil {
			return map[string]string{}, cleanupParseError(filename, err)
		}
	}

	rendered = make(map[string]string, len(keys))
	for _, filename := range keys {
		// Don't render partials. We don't care out the direct output of partials.
		// They are only included from other templates.
		if strings.HasPrefix(path.Base(filename), "_") {
			continue
		}
		// At render time, add information about the template that is being rendered.
		vals := tpls[filename].vals
		vals["Template"] = chartutil.Values{"Name": filename, "BasePath": tpls[filename].basePath}
		if values, ok := vals["Values"]; !ok || values == nil {
			vals["Values"] = make(chartutil.Values)
		}
		var buf strings.Builder
		executeErr := t.ExecuteTemplate(&buf, filename, vals)

		if executeErr != nil {
			// Check for specific recoverable nil pointer errors from template execution
			isRecoverableNilError := false
			errStr := executeErr.Error()
			if strings.Contains(errStr, "nil pointer evaluating") || // Error from text/template like ".nilInterface.field"
				strings.Contains(errStr, "invalid memory address or nil pointer dereference") { // General runtime error for nil dereference
				isRecoverableNilError = true
			}

			if e.LintMode && isRecoverableNilError {
				logger.ErrorF("[LINT] Template %s encountered a nil pointer access during execution: %v. Using partially rendered output.", filename, executeErr)
				rendered[filename] = getRenderedContent(&buf)
			} else {
				// For other errors, or if not in LintMode, this is a hard error.
				return map[string]string{}, cleanupExecError(filename, executeErr)
			}
		} else {
			// No error during template execution.
			// Work around the issue where Go will emit "<no value>" even if Options(missing=zero)
			// is set. Since missing=error will never get here, we do not need to handle
			// the Strict case.
			rendered[filename] = getRenderedContent(&buf)
		}
	}

	return rendered, nil
}

func cleanupParseError(filename string, err error) error {
	tokens := strings.Split(err.Error(), ": ")
	if len(tokens) == 1 {
		// This might happen if a non-templating error occurs
		return fmt.Errorf("parse error in (%s): %w", filename, err)
	}
	// The first token is "template"
	// The second token is either "filename:lineno" or "filename:lineNo:columnNo"
	location := tokens[1]
	// The remaining tokens make up a stacktrace-like chain, ending with the relevant error
	errMsg := tokens[len(tokens)-1]
	return fmt.Errorf("parse error at (%s): %s", location, errMsg)
}

func cleanupExecError(filename string, err error) error {
	if _, isExecError := err.(template.ExecError); !isExecError {
		return err
	}

	tokens := strings.SplitN(err.Error(), ": ", 3)
	if len(tokens) != 3 {
		// This might happen if a non-templating error occurs
		return fmt.Errorf("execution error in (%s): %w", filename, err)
	}

	// The first token is "template"
	// The second token is either "filename:lineno" or "filename:lineNo:columnNo"
	location := tokens[1]

	parts := warnRegex.FindStringSubmatch(tokens[2])
	if len(parts) >= 2 {
		return fmt.Errorf("execution error at (%s): %s", location, parts[1])
	}

	return err
}

func sortTemplates(tpls map[string]renderable) []string {
	keys := make([]string, len(tpls))
	i := 0
	for key := range tpls {
		keys[i] = key
		i++
	}
	sort.Sort(sort.Reverse(byPathLen(keys)))
	return keys
}

type byPathLen []string

func (p byPathLen) Len() int      { return len(p) }
func (p byPathLen) Swap(i, j int) { p[j], p[i] = p[i], p[j] }
func (p byPathLen) Less(i, j int) bool {
	a, b := p[i], p[j]
	ca, cb := strings.Count(a, "/"), strings.Count(b, "/")
	if ca == cb {
		return a < b
	}
	return ca < cb
}

// allTemplates returns all templates for a chart and its dependencies.
//
// As it goes, it also prepares the values in a scope-sensitive manner.
func allTemplates(c *chart.Chart, vals chartutil.Values) map[string]renderable {
	templates := make(map[string]renderable)
	recAllTpls(c, templates, vals)
	return templates
}

// recAllTpls recurses through the templates in a chart.
//
// As it recurses, it also sets the values to be appropriate for the template
// scope.
func recAllTpls(c *chart.Chart, templates map[string]renderable, vals chartutil.Values) map[string]any {
	subCharts := make(map[string]any)
	chartMetaData := struct {
		chart.Metadata
		IsRoot bool
	}{*c.Metadata, c.IsRoot()}

	next := map[string]any{
		"Chart":        chartMetaData,
		"Files":        newFiles(c.Files),
		"Release":      vals["Release"],
		"Capabilities": vals["Capabilities"],
		"Values":       make(chartutil.Values),
		"Subcharts":    subCharts,
	}

	// If there is a {{.Values.ThisChart}} in the parent metadata,
	// copy that into the {{.Values}} for this template.
	if c.IsRoot() {
		next["Values"] = vals["Values"]
	} else if vs, err := vals.Table("Values." + c.Name()); err == nil {
		next["Values"] = vs
	}

	for _, child := range c.Dependencies() {
		subCharts[child.Name()] = recAllTpls(child, templates, next)
	}

	newParentID := c.ChartFullPath()
	for _, t := range c.Templates {
		if t == nil {
			continue
		}
		if !isTemplateValid(c, t.Name) {
			continue
		}
		templates[path.Join(newParentID, t.Name)] = renderable{
			tpl:      string(t.Data),
			vals:     next,
			basePath: path.Join(newParentID, "templates"),
		}
	}

	return next
}

// isTemplateValid returns true if the template is valid for the chart type
func isTemplateValid(ch *chart.Chart, templateName string) bool {
	if isLibraryChart(ch) {
		return strings.HasPrefix(filepath.Base(templateName), "_")
	}
	return true
}

// isLibraryChart returns true if the chart is a library chart
func isLibraryChart(c *chart.Chart) bool {
	return strings.EqualFold(c.Metadata.Type, "library")
}
