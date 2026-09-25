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

package bootstrap

import (
	"regexp"
	"sort"
	"strings"
	"text/template/parse"
)

// Doc is one YAML document of a template and the template blocks around it: what the render
// cannot show, because it only holds what rendered for the linter's values.
type Doc struct {
	// Kind, Name and Namespace are the literal values of the document. A name the template computes
	// is in NameTemplate as written, and NamePattern matches the names it can produce; a document
	// whose name cannot be read has neither and matches nothing.
	Kind, Name, Namespace string
	NameTemplate          string
	NamePattern           *regexp.Regexp
	// Library marks a document without an object of its own that includes a named template: the
	// objects it renders come from helm_lib or another chart.
	Library bool
	// When is the condition under which the document renders, as a `when` expression.
	When string
	// Unmanageable says why the declaration cannot carry the condition: a range, a with, a define.
	Unmanageable string
	// Partial marks a block that opens and closes inside the object: part of it is conditional.
	Partial bool
}

var (
	docSeparatorRe = regexp.MustCompile(`(?m)^---[ \t]*(#.*)?$`)
	docKindRe      = regexp.MustCompile(`(?m)^kind:[ \t]*["']?([A-Za-z]+)["']?[ \t]*(#.*)?$`)
	metadataRe     = regexp.MustCompile(`(?m)^metadata:[ \t]*(\{.*\})?[ \t]*(#.*)?$`)
	blockFieldRe   = regexp.MustCompile(`^([ \t]+)(name|namespace):[ \t]*(.*?)[ \t]*$`)
	flowFieldRe    = regexp.MustCompile(`(name|namespace):[ \t]*("[^"]*"|'[^']*'|\{\{.*?\}\}[^,}]*|[^,}\s]+)`)
	nameActionRe   = regexp.MustCompile(`\{\{.*?\}\}`)
	variableRe     = regexp.MustCompile(`\$[A-Za-z_]`)
)

// frame is an open block of the template around a piece of text.
type frame struct {
	kind string // if, else, range, with, define
	cond string // the if's pipeline, as the parser prints it
}

// span is a piece of the template the parser saw, with the blocks open around it.
type span struct {
	start, end int
	stack      []frame
	include    bool // an include or template action
}

// block is a control structure with the extent of its body.
type block struct {
	start, end int
}

// reader collects what the parser saw in one template.
type reader struct {
	spans  []span
	blocks []block
}

// TemplateDocs reads the documents of a template and the blocks around each, with the template
// parser of the standard library. What it cannot follow ends up as Unmanageable or as a TODO in the
// declaration; a template that does not parse yields no documents.
func TemplateDocs(text string) []Doc {
	text = strings.ReplaceAll(text, "\r\n", "\n")

	trees := map[string]*parse.Tree{}

	root := parse.New("template")
	root.Mode = parse.SkipFuncCheck

	if _, err := root.Parse(text, "", "", trees); err != nil {
		return nil
	}

	r := &reader{}

	if _, ok := trees["template"]; !ok {
		trees["template"] = root
	}

	for name, tree := range trees {
		if tree == nil || tree.Root == nil {
			continue
		}

		var stack []frame
		if name != "template" {
			stack = []frame{{kind: "define"}}
		}

		r.walk(tree.Root, stack)
	}

	sort.Slice(r.spans, func(i, j int) bool { return r.spans[i].start < r.spans[j].start })

	return r.docs(text)
}

// walk records the text, actions and blocks under node and returns the end of what it saw.
func (r *reader) walk(node parse.Node, stack []frame) int {
	end := 0

	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return 0
		}

		for _, c := range n.Nodes {
			end = max(end, r.walk(c, stack))
		}
	case *parse.TextNode:
		start := int(n.Position())
		r.spans = append(r.spans, span{start: start, end: start + len(n.Text), stack: stack})

		return start + len(n.Text)
	case *parse.ActionNode:
		start := int(n.Position())
		r.spans = append(r.spans, span{start: start, end: start, stack: stack, include: includes(n.Pipe)})

		return start
	case *parse.TemplateNode:
		start := int(n.Position())
		r.spans = append(r.spans, span{start: start, end: start, stack: stack, include: true})

		return start
	case *parse.IfNode:
		cond := n.Pipe.String()
		end = max(r.walk(n.List, push(stack, frame{kind: "if", cond: cond})), r.walk(n.ElseList, push(stack, frame{kind: "else", cond: cond})))
		r.blocks = append(r.blocks, block{start: int(n.Position()), end: end})
	case *parse.RangeNode:
		end = max(r.walk(n.List, push(stack, frame{kind: "range"})), r.walk(n.ElseList, push(stack, frame{kind: "range"})))
		r.blocks = append(r.blocks, block{start: int(n.Position()), end: end})
	case *parse.WithNode:
		end = max(r.walk(n.List, push(stack, frame{kind: "with"})), r.walk(n.ElseList, push(stack, frame{kind: "with"})))
		r.blocks = append(r.blocks, block{start: int(n.Position()), end: end})
	}

	return end
}

func push(stack []frame, f frame) []frame {
	out := make([]frame, len(stack), len(stack)+1)
	copy(out, stack)

	return append(out, f)
}

// includes reports whether a pipeline calls include or template.
func includes(pipe *parse.PipeNode) bool {
	if pipe == nil {
		return false
	}

	for _, cmd := range pipe.Cmds {
		if len(cmd.Args) > 0 {
			if id, ok := cmd.Args[0].(*parse.IdentifierNode); ok && (id.Ident == "include" || id.Ident == "tpl") {
				return true
			}
		}
	}

	return false
}

// textAt returns the span of text that holds offset, if any: offsets inside actions and comments
// are not text.
func (r *reader) textAt(offset int) (span, bool) {
	i := sort.Search(len(r.spans), func(i int) bool { return r.spans[i].end > offset })
	for ; i < len(r.spans) && r.spans[i].start <= offset; i++ {
		if r.spans[i].end > offset {
			return r.spans[i], true
		}
	}

	return span{}, false
}

func (r *reader) docs(text string) []Doc {
	// A separator counts only where the parser saw text: one inside a comment separates nothing.
	var bounds [][2]int

	for _, m := range docSeparatorRe.FindAllStringIndex(text, -1) {
		if _, ok := r.textAt(m[0]); ok {
			bounds = append(bounds, [2]int{m[0], m[1]})
		}
	}

	bounds = append(bounds, [2]int{len(text), len(text)})

	var (
		out   []Doc
		start int
	)

	for _, b := range bounds {
		docStart, docEnd := start, b[0]
		start = b[1]

		if d, ok := r.doc(text, docStart, docEnd); ok {
			out = append(out, d)
		}
	}

	return out
}

func (r *reader) doc(text string, docStart, docEnd int) (Doc, bool) {
	body := text[docStart:docEnd]

	kindAt := -1

	var d Doc

	for _, m := range docKindRe.FindAllStringSubmatchIndex(body, -1) {
		if _, ok := r.textAt(docStart + m[0]); ok {
			d.Kind, kindAt = body[m[2]:m[3]], docStart+m[0]
			break
		}
	}

	if kindAt < 0 {
		// A document with no object of its own that includes a named template renders what the
		// template holds.
		for _, s := range r.spans {
			if s.include && s.start >= docStart && s.start < docEnd {
				d.Library = true
				d.When, d.Unmanageable = conditionOf(s.stack)

				return d, true
			}
		}

		return Doc{}, false
	}

	at, _ := r.textAt(kindAt)
	d.When, d.Unmanageable = conditionOf(at.stack)
	d.Name, d.Namespace, d.NameTemplate = metadataOf(body)

	if d.NameTemplate != "" {
		d.NamePattern = namePattern(d.NameTemplate)
	}

	// A block that opens and closes inside the object gates part of it; one that opens here and
	// stays open belongs to the documents after it.
	for _, blk := range r.blocks {
		if blk.start > kindAt && blk.start < docEnd && blk.end <= docEnd {
			d.Partial = true
		}
	}

	return d, true
}

// conditionOf turns the open blocks into a `when`: an if branch is its condition, an else the
// negation of it, nested blocks an `and` of them. Every argument of and is parenthesized, a
// negation too: `and not (a) (b)` parses, yet passes not as a value and fails when it renders.
func conditionOf(stack []frame) (string, string) {
	var parts []string

	for _, f := range stack {
		switch f.kind {
		case "range":
			return "", "rendered inside {{ range }}, which the declaration cannot express"
		case "with":
			return "", "rendered inside {{ with }}, which changes the dot the declaration's `when` is written against"
		case "define":
			return "", "rendered from a named template ({{ define }}), which the declaration cannot express"
		case "if":
			parts = append(parts, f.cond)
		case "else":
			parts = append(parts, "not ("+unwrap(f.cond)+")")
		}
	}

	var when string

	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		when = parts[0]
	default:
		for i, p := range parts {
			parts[i] = "(" + unwrap(p) + ")"
		}

		when = "and " + strings.Join(parts, " ")
	}

	// A variable of the template does not exist where the generator writes the condition.
	if variableRe.MatchString(when) {
		return "TODO: " + when + " -- the template condition uses a variable of the template; write it against the root values", ""
	}

	// The declaration's `when` holds no delimiter: the generator writes it inside {{ if }}.
	if strings.Contains(when, "{{") || strings.Contains(when, "}}") {
		return "TODO: " + when + " -- the template condition holds a template delimiter, which `when` cannot; rewrite it without one", ""
	}

	return when, ""
}

// metadataOf reads the name and namespace of a document: block style at any indentation, or flow
// style. A computed name is returned as written.
func metadataOf(body string) (string, string, string) {
	m := metadataRe.FindStringSubmatchIndex(body)
	if m == nil {
		return "", "", ""
	}

	fields := map[string]string{}

	if m[2] >= 0 {
		for _, f := range flowFieldRe.FindAllStringSubmatch(body[m[2]:m[3]], -1) {
			if _, seen := fields[f[1]]; !seen {
				fields[f[1]] = f[2]
			}
		}
	} else {
		// The fields of metadata are the lines at the indentation of its first field, up to the
		// first line back at column 0.
		indent := ""

		for _, line := range strings.Split(body[m[1]:], "\n")[1:] {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "{{") || strings.HasPrefix(trimmed, "#") {
				continue
			}

			lead := len(line) - len(strings.TrimLeft(line, " \t"))
			if lead == 0 {
				break
			}

			if indent == "" {
				indent = line[:lead]
			}

			if f := blockFieldRe.FindStringSubmatch(line); f != nil && f[1] == indent {
				if _, seen := fields[f[2]]; !seen {
					fields[f[2]] = f[3]
				}
			}
		}
	}

	name, template := literal(fields["name"])
	namespace, _ := literal(fields["namespace"])

	return name, namespace, template
}

// literal returns a YAML scalar as a literal, or, when the template computes it, "" and the
// value as written.
func literal(value string) (string, string) {
	if !strings.Contains(value, "{{") {
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}

		return strings.Trim(value, `"'`), ""
	}

	return "", strings.Trim(value, `"'`)
}

// namePattern matches the names a templated name can produce: its literal parts in place, any
// text for each action.
func namePattern(value string) *regexp.Regexp {
	var b strings.Builder

	b.WriteString("^")

	last := 0
	for _, m := range nameActionRe.FindAllStringIndex(value, -1) {
		b.WriteString(regexp.QuoteMeta(value[last:m[0]]))
		b.WriteString(".*")

		last = m[1]
	}

	b.WriteString(regexp.QuoteMeta(value[last:]))
	b.WriteString("$")

	return regexp.MustCompile(b.String())
}

// unwrap drops parentheses around a whole expression: (a | b) -> a | b.
func unwrap(expr string) string {
	for len(expr) > 1 && expr[0] == '(' && expr[len(expr)-1] == ')' {
		depth := 0

		for i, c := range expr {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
			}

			if depth == 0 && i < len(expr)-1 {
				return expr // the first parenthesis closes before the end: (a) and (b)
			}
		}

		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}

	return expr
}

// Locate finds the document of a rendered object among the documents of its template. An object
// found in no document of its own kind but in a file that includes a named template came from
// that template (helm_lib, typically). ok is false when the text does not tell.
func Locate(docs []Doc, o Object) (Doc, bool) {
	var byName, templated []Doc

	for _, d := range docs {
		// A namespace the text names must be the object's.
		if d.Kind != o.Kind || (d.Namespace != "" && o.Namespace != "" && d.Namespace != o.Namespace) {
			continue
		}

		switch {
		case d.Name != "" && d.Name == o.Name:
			byName = append(byName, d)
		case d.NamePattern != nil && d.NamePattern.MatchString(o.Name):
			templated = append(templated, d)
		}
	}

	for _, set := range [][]Doc{byName, templated} {
		if len(set) == 0 {
			continue
		}

		// Several candidates are one answer only when their blocks agree.
		for _, d := range set[1:] {
			if d.When != set[0].When || d.Unmanageable != set[0].Unmanageable {
				return Doc{}, false
			}
		}

		return set[0], true
	}

	// The library documents are one answer only when their blocks agree too: an include under a
	// condition beside one without would give its objects no condition (review of #480).
	var library []Doc

	for _, d := range docs {
		if d.Library {
			library = append(library, d)
		}
	}

	if len(library) == 0 {
		return Doc{}, false
	}

	for _, d := range library[1:] {
		if d.When != library[0].When || d.Unmanageable != library[0].Unmanageable {
			return Doc{}, false
		}
	}

	return library[0], true
}
