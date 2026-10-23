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

package generate

import (
	"sort"
	"strings"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
)

// Rendered is the text of one generated file.
type Rendered struct {
	Path    string
	Content string
}

// Render writes every file of the model as a Helm template. The output is a pure function of the
// model: the same declaration always renders the same bytes.
func Render(m *Model) []Rendered {
	out := make([]Rendered, 0, len(m.Files))
	for _, f := range m.Files {
		out = append(out, Rendered{Path: f.Path, Content: RenderFile(f)})
	}

	return out
}

// AggregationLabels returns the (lineage, level) pairs of a capability, sorted by lineage.
func (o Object) AggregationLabels() []LineageLevel {
	out := make([]LineageLevel, 0, len(o.Labels))

	for key, value := range o.Labels {
		if strings.HasPrefix(key, rbaccontract.AggregationLabelPrefix) && strings.HasSuffix(key, rbaccontract.AggregationLabelSuffix) {
			out = append(out, LineageLevel{
				Lineage: strings.TrimSuffix(strings.TrimPrefix(key, rbaccontract.AggregationLabelPrefix), rbaccontract.AggregationLabelSuffix),
				Level:   value,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Lineage < out[j].Lineage })

	return out
}

// Paths returns the generated paths in order.
func (m *Model) Paths() []string {
	out := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		out = append(out, f.Path)
	}

	return out
}

// LineageLevel is one aggregation edge of a capability.
type LineageLevel struct {
	Lineage string
	Level   string
}
