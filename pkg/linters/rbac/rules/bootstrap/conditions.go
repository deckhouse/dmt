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
	"strings"
)

// Doc is one YAML document of a template and the template blocks around it: what the render
// cannot show, because it only holds what rendered for the linter's values.
type Doc struct {
	// Kind, Name and Namespace are the literal values of the document; Name is empty when the
	// template computes it.
	Kind, Name, Namespace string
	// Library marks a document without an object of its own that includes a named template: the
	// objects it renders come from helm_lib or another chart.
	Library bool
	// When is the condition under which the document renders, as a `when` expression.
	When string
	// Unmanageable says why the declaration cannot carry the condition: a range, a with, a define.
	Unmanageable string
	// Partial marks a block that opens inside the object: part of it is conditional.
	Partial bool
}

var (
	docSeparatorRe = regexp.MustCompile(`(?m)^---[ \t]*(#.*)?$`)
	actionRe       = regexp.MustCompile(`(?s)\{\{-?(.*?)-?\}\}`)
	docKindRe      = regexp.MustCompile(`(?m)^kind:[ \t]*(\S+)[ \t]*$`)
	docMetadataRe  = regexp.MustCompile(`(?m)^metadata:[ \t]*$`)
	docFieldRe     = regexp.MustCompile(`^  (name|namespace):[ \t]*(.+?)[ \t]*$`)
	includeRe      = regexp.MustCompile(`\{\{-?\s*(include|template)\s+"`)
	variableRe     = regexp.MustCompile(`\$[A-Za-z_]`)
)

// frame is an open template block. For an if, cur is the condition of the branch being read and
// prior the conditions of the branches before it.
type frame struct {
	kind  string
	cur   string
	prior []string
}

type action struct {
	offset int
	words  []string
	body   string
}

// TemplateDocs reads the documents of a template and the blocks around each. It is a reader of
// the common shapes -- an object wrapped in {{ if }}, {{ else }}, {{ range }}, {{ with }} -- not a
// template engine: what it cannot follow ends up as Unmanageable or as a TODO in the declaration.
func TemplateDocs(text string) []Doc {
	text = strings.ReplaceAll(text, "\r\n", "\n")

	actions := templateActions(text)

	var (
		out   []Doc
		stack []frame
		next  int
		start int
	)

	bounds := docSeparatorRe.FindAllStringIndex(text, -1)
	bounds = append(bounds, []int{len(text), len(text)})

	for _, b := range bounds {
		doc := text[start:b[0]]
		docStart, docEnd := start, b[0]
		start = b[1]

		d := Doc{}
		at := docEnd

		if m := docKindRe.FindStringSubmatchIndex(doc); m != nil {
			d.Kind = doc[m[2]:m[3]]
			at = docStart + m[0]
		}

		// The blocks open at the object's kind line are the object's condition.
		for next < len(actions) && actions[next].offset < at {
			stack = apply(stack, actions[next])
			next++
		}

		if d.Kind == "" {
			if includeRe.MatchString(doc) && strings.TrimSpace(actionRe.ReplaceAllString(doc, "")) == "" {
				d.Library = true
				d.When, d.Unmanageable = conditionOf(stack)
				out = append(out, d)
			}

			for next < len(actions) && actions[next].offset < docEnd {
				stack = apply(stack, actions[next])
				next++
			}

			continue
		}

		d.Name, d.Namespace = metadataOf(doc)
		d.When, d.Unmanageable = conditionOf(stack)

		for next < len(actions) && actions[next].offset < docEnd {
			if w := actions[next].words; len(w) > 0 && (w[0] == "if" || w[0] == "range" || w[0] == "with") {
				d.Partial = true
			}

			stack = apply(stack, actions[next])
			next++
		}

		out = append(out, d)
	}

	return out
}

func templateActions(text string) []action {
	var out []action

	for _, m := range actionRe.FindAllStringSubmatchIndex(text, -1) {
		body := strings.TrimSpace(text[m[2]:m[3]])
		if strings.HasPrefix(body, "/*") {
			continue
		}

		out = append(out, action{offset: m[0], words: strings.Fields(body), body: body})
	}

	return out
}

func apply(stack []frame, a action) []frame {
	if len(a.words) == 0 {
		return stack
	}

	switch a.words[0] {
	case "if":
		return append(stack, frame{kind: "if", cur: strings.TrimSpace(strings.TrimPrefix(a.body, "if"))})
	case "range", "with", "define", "block":
		return append(stack, frame{kind: a.words[0]})
	case "else":
		if len(stack) == 0 {
			return stack
		}

		top := &stack[len(stack)-1]
		if top.kind != "if" {
			// {{ else }} of a range or a with: still not something the declaration expresses.
			return stack
		}

		top.prior = append(top.prior, top.cur)

		rest := strings.TrimSpace(strings.TrimPrefix(a.body, "else"))
		switch {
		case strings.HasPrefix(rest, "if "):
			top.cur = strings.TrimSpace(strings.TrimPrefix(rest, "if"))
		case rest == "":
			top.cur = ""
		default:
			// else with ...: the dot changes.
			top.kind = "with"
		}
	case "end":
		if len(stack) > 0 {
			return stack[:len(stack)-1]
		}
	}

	return stack
}

// conditionOf turns the open blocks into a `when`: an if branch is its condition, an else the
// negation of the branches before it, nested blocks an `and` of them.
func conditionOf(stack []frame) (string, string) {
	var parts []string

	for _, f := range stack {
		switch f.kind {
		case "range":
			return "", "rendered inside {{ range }}, which the declaration cannot express"
		case "with":
			return "", "rendered inside {{ with }}, which changes the dot the declaration's `when` is written against"
		case "define", "block":
			return "", "rendered from a named template ({{ define }}), which the declaration cannot express"
		}

		for _, p := range f.prior {
			parts = append(parts, "not ("+unwrap(p)+")")
		}

		if f.cur != "" {
			parts = append(parts, f.cur)
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
			if !strings.HasPrefix(p, "not (") {
				parts[i] = "(" + unwrap(p) + ")"
			}
		}

		when = "and " + strings.Join(parts, " ")
	}

	// A variable of the template does not exist where the generator writes the condition.
	if variableRe.MatchString(when) || strings.Contains(when, "{{") || strings.Contains(when, "}}") {
		return "TODO: " + when + " -- the template condition uses a variable of the template; write it against the root values", ""
	}

	return when, ""
}

func metadataOf(doc string) (string, string) {
	m := docMetadataRe.FindStringIndex(doc)
	if m == nil {
		return "", ""
	}

	var name, namespace string

	for _, line := range strings.Split(doc[m[1]:], "\n")[1:] {
		if !strings.HasPrefix(line, "  ") {
			break
		}

		f := docFieldRe.FindStringSubmatch(line)
		if f == nil {
			continue
		}

		value := strings.Trim(f[2], `"'`)
		if strings.Contains(value, "{{") {
			value = ""
		}

		switch {
		case f[1] == "name" && name == "":
			name = value
		case f[1] == "namespace" && namespace == "":
			namespace = value
		}
	}

	return name, namespace
}

// Locate finds the document of a rendered object among the documents of its template. An object
// found in no document of its own kind but in a file that includes a named template came from
// that template (helm_lib, typically). ok is false when the text does not tell.
func Locate(docs []Doc, o Object) (Doc, bool) {
	var byName, templated []Doc

	for _, d := range docs {
		if d.Kind != o.Kind {
			continue
		}

		switch d.Name {
		case o.Name:
			byName = append(byName, d)
		case "":
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

	for _, d := range docs {
		if d.Library {
			return d, true
		}
	}

	return Doc{}, false
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
