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

// Package matrix expands a module's openapi value schema into many concrete
// value combinations ("variants"). A single default render only exercises one
// branch of a chart's templates; matrix mode renders every combination so that
// conditionally-rendered resources (guarded by feature flags, modes, enums,
// etc.) are produced and linted too.
//
// Axes of variation are discovered from the module's own openapi schema:
//
//   - a node carrying x-examples contributes one variant per example;
//   - an enum contributes one variant per allowed value;
//   - a boolean contributes true and false.
//
// Combinations are the cartesian product of all axes, capped by a limit. When
// the product exceeds the limit, an all-pairs (pairwise) set is produced
// instead so that every pair of axis values still co-occurs in some variant —
// enough to reach bugs that need two conditions at once (e.g. "feature enabled"
// AND "mode = Static").
package matrix

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-openapi/spec"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/values"
)

// xExamples is the openapi extension holding example values for a node.
const xExamples = "x-examples"

// defaultLimit caps the number of combinations when the caller passes a
// non-positive limit (e.g. tests that don't parse CLI flags).
const defaultLimit = 100

// Axis is one dimension of variation: the value at Path may take any of Values.
type Axis struct {
	Path   []string
	Values []any
}

// Variant is a set of value overrides to render, structured as the chart's
// .Values tree (i.e. keyed by the camel-cased module name). A nil/empty Variant
// means "render with the default generated values".
type Variant struct {
	// Overrides is the .Values override tree ({camelName: {...}}); nil for the
	// default variant.
	Overrides chartutil.Values
	// Label is a short human-readable description of what this variant sets.
	Label string
}

// Generate returns the value variants to render for the module at modulePath,
// using valuesFile (e.g. "values.yaml") as the openapi values schema. The first
// variant is always the default (no overrides), so matrix output is a superset
// of a normal lint. At most limit combinations are produced.
func Generate(modulePath, valuesFile string, limit int) ([]Variant, error) {
	if limit <= 0 {
		limit = defaultLimit
	}

	camelName, err := moduleCamelName(modulePath)
	if err != nil {
		return nil, err
	}

	schema, err := values.GetModuleValuesForValuesFile(modulePath, valuesFile)
	if err != nil {
		return nil, fmt.Errorf("load module values schema: %w", err)
	}

	axes := discoverAxes(schema)

	// Always start with the default (no-override) render.
	variants := []Variant{{Overrides: nil, Label: "default"}}

	if len(axes) == 0 {
		return variants, nil
	}

	combos := combinations(axes, max(1, limit-1))

	for _, combo := range combos {
		variants = append(variants, Variant{
			Overrides: buildOverride(camelName, axes, combo),
			Label:     comboLabel(axes, combo),
		})
	}

	return variants, nil
}

// discoverAxes walks a module values schema and collects one Axis per node that
// offers a finite, meaningful set of alternative values.
func discoverAxes(schema *spec.Schema) []Axis {
	var axes []Axis

	walkSchema(schema, nil, &axes)

	return axes
}

func walkSchema(s *spec.Schema, path []string, axes *[]Axis) {
	if s == nil {
		return
	}

	// A node's own alternatives come from its x-examples and its oneOf branches;
	// each becomes a candidate value for the whole subtree. We still recurse into
	// the node's properties afterwards so nested enums/booleans are varied too —
	// every combination is exercised even when the author provided examples.
	if nodeValues := collectNodeValues(s); len(nodeValues) > 1 {
		addAxis(axes, path, nodeValues)
	} else if len(s.Enum) > 1 {
		addAxis(axes, path, append([]any{}, s.Enum...))
		return // scalar leaf: nothing to recurse into
	} else if s.Type.Contains("boolean") {
		addAxis(axes, path, []any{true, false})
		return // scalar leaf
	}

	for _, key := range sortedKeys(s.Properties) {
		child := s.Properties[key]
		walkSchema(&child, append(path, key), axes)
	}
}

// collectNodeValues gathers whole-subtree candidate values for a node: its
// x-examples plus one generated value per oneOf branch. Expanding oneOf lets the
// matrix reach each alternative shape a field can take (e.g. mode: VPA vs mode:
// Static), not just the one the default generator happens to pick.
func collectNodeValues(s *spec.Schema) []any {
	var vals []any

	vals = append(vals, schemaExamples(s)...)

	for i := range s.OneOf {
		if v, ok := branchValue(s, &s.OneOf[i]); ok {
			vals = append(vals, v)
		}
	}

	return vals
}

// branchValue generates a representative value for a single oneOf branch by
// overlaying the branch's properties on the parent's and running the shared
// openapi value generator. Returns false when nothing could be generated.
func branchValue(parent, branch *spec.Schema) (any, bool) {
	merged := spec.Schema{}
	merged.Properties = make(map[string]spec.Schema, len(parent.Properties)+len(branch.Properties))

	for k := range parent.Properties {
		merged.Properties[k] = parent.Properties[k]
	}

	for k := range branch.Properties {
		merged.Properties[k] = branch.Properties[k]
	}

	if len(merged.Properties) == 0 {
		return nil, false
	}

	generated, err := module.NewOpenAPIValuesGenerator(&merged).Do()
	if err != nil || len(generated) == 0 {
		return nil, false
	}

	return generated, true
}

// addAxis appends an axis at path. If an axis already exists there (branches and
// properties can reference the same location), the values are merged rather than
// dropped, so no variant is lost.
func addAxis(axes *[]Axis, path []string, valuesList []any) {
	target := pathString(path)

	for i := range *axes {
		if pathString((*axes)[i].Path) == target {
			(*axes)[i].Values = append((*axes)[i].Values, valuesList...)
			return
		}
	}

	*axes = append(*axes, Axis{Path: clonePath(path), Values: valuesList})
}

// schemaExamples extracts the x-examples of a node, normalized to []any.
func schemaExamples(s *spec.Schema) []any {
	raw, ok := s.Extensions[xExamples]
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for i := range v {
			out = append(out, v[i])
		}

		return out
	default:
		return nil
	}
}

// buildOverride turns one combination (an index per axis) into a .Values
// override tree keyed by the module's camel name.
func buildOverride(camelName string, axes []Axis, combo []int) chartutil.Values {
	moduleValues := map[string]any{}

	// Apply shallower paths first so a whole-subtree value (e.g. a oneOf branch
	// or an x-example at resourcesRequests) is written before a nested override
	// (e.g. resourcesRequests.mode) that must land inside it.
	order := make([]int, len(combo))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(a, b int) bool {
		return len(axes[order[a]].Path) < len(axes[order[b]].Path)
	})

	for _, i := range order {
		setPath(moduleValues, axes[i].Path, deepCopyValue(axes[i].Values[combo[i]]))
	}

	return chartutil.Values{camelName: moduleValues}
}

func comboLabel(axes []Axis, combo []int) string {
	parts := make([]string, len(combo))
	for i, valueIdx := range combo {
		parts[i] = fmt.Sprintf("%s=%v", pathString(axes[i].Path), axes[i].Values[valueIdx])
	}

	return strings.Join(parts, ", ")
}

// setPath assigns value at the nested key path within root, creating
// intermediate maps as needed.
func setPath(root map[string]any, path []string, value any) {
	node := root

	for i, key := range path {
		if i == len(path)-1 {
			node[key] = value
			return
		}

		next, ok := node[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			node[key] = next
		}

		node = next
	}
}

func moduleCamelName(modulePath string) (string, error) {
	moduleYaml, err := module.ParseModuleConfigFile(modulePath)
	if err != nil {
		return "", fmt.Errorf("parse module.yaml: %w", err)
	}

	chartYaml, err := module.ParseChartFile(modulePath)
	if err != nil {
		return "", fmt.Errorf("parse Chart.yaml: %w", err)
	}

	name := module.GetModuleName(moduleYaml, chartYaml)
	if name == "" {
		return "", fmt.Errorf("module at %q has no name", modulePath)
	}

	return module.ToLowerCamel(name), nil
}

func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = deepCopyValue(val)
		}

		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = deepCopyValue(t[i])
		}

		return out
	default:
		return v
	}
}

func clonePath(path []string) []string {
	out := make([]string, len(path))
	copy(out, path)

	return out
}

func pathString(path []string) string {
	return strings.Join(path, ".")
}

func sortedKeys(m map[string]spec.Schema) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
