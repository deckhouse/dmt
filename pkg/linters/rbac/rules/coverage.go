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
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

const (
	CoverageRuleName = "coverage"

	// FixCommand is what closes a coverage or sync finding that carries an autofix.
	FixCommand = "dmt lint --linter rbac --fix"
)

// CoverageRule requires a decision on the user access to every CRD the module ships: an entry
// in rbac.yaml that grants levels or denies access with a reason. It runs only when the module
// has an rbac.yaml (spec 005 R22); without one, only the contract rule applies.
//
// Its autofix appends an undecided stub (noAccess: "TODO") for each CRD without an entry and
// then reports that a decision is still owed, so a --fix run that wrote stubs does not end
// green (R33): the tool never takes the decision for the author (R10).
type CoverageRule struct {
	pkg.RuleMeta
	pkg.StringRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*CoverageRule)(nil)

// NewCoverageRule builds the rule. excludeRules lists "group/resource" keys of CRDs the module
// deliberately keeps out of the declaration.
func NewCoverageRule(excludeRules []pkg.StringRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *CoverageRule {
	return &CoverageRule{
		RuleMeta:   pkg.RuleMeta{Name: CoverageRuleName},
		StringRule: pkg.StringRule{ExcludeRules: excludeRules},
		module:     m,
		errorList:  errorList.WithRule(CoverageRuleName),
	}
}

func (r *CoverageRule) Check(_ context.Context) {
	modulePath := r.module.GetPath()
	errorList := r.errorList.WithFilePath(rbacyaml.Filename)

	decl, err := rbacyaml.Load(modulePath)
	if err != nil || editionOverlay(modulePath) != "" {
		// No declaration: nothing to cover (R22). A declaration that does not parse, or that lies
		// in an edition overlay (D7), is the sync rule's finding; reporting it twice would only
		// double the noise.
		return
	}

	crds, skipped := moduleCRDs(modulePath)
	for _, err := range skipped {
		r.errorList.WithFilePath("crds").Warnf("a CRD document is skipped: %v; its resource is judged as external until it parses", err)
	}

	entries := make(map[string]rbacyaml.Resource, len(decl.Resources))
	for _, res := range decl.Resources {
		entries[res.Key()] = res
	}

	groups := make(map[string]struct{}, len(crds))
	known := make(map[string]struct{}, len(crds))

	for _, crd := range crds {
		groups[crd.Group] = struct{}{}
		known[crd.Key()] = struct{}{}

		if !r.Enabled(crd.Key()) {
			continue
		}

		entry, ok := entries[crd.Key()]
		if !ok {
			errorList.
				WithObjectID("CustomResourceDefinition/"+crd.Key()).
				WithFix(appendStubFix(modulePath, crd)).
				Errorf("CRD %s (%s) has no entry in %s: decide the user access to it -- namespace, system or legacy levels, or noAccess with the reason; `%s` adds an undecided stub",
					crd.Key(), crd.File, rbacyaml.Filename, FixCommand)

			continue
		}

		if entry.NoAccess == rbacyaml.NoAccessTODO {
			errorList.
				WithObjectID("CustomResourceDefinition/"+crd.Key()).
				Errorf("%s is still noAccess: %q in %s: a decision is needed -- grant levels, or replace %q with the reason users get no access; only a person can close this",
					crd.Key(), rbacyaml.NoAccessTODO, rbacyaml.Filename, rbacyaml.NoAccessTODO)
		}
	}

	// A resource of a group the module ships CRDs for, but not one of them, is most likely a
	// misspelling (R11). Whole-group and subresource entries are exempt: CRDs describe neither.
	for _, res := range decl.Resources {
		if res.IsWildcard() || res.IsSubresource() || !r.Enabled(res.Key()) {
			continue
		}

		if _, groupKnown := groups[res.Group]; !groupKnown {
			// A denied resource of a group the module ships no CRD for: either an external resource
			// nobody grants, or a CRD that was removed while its entry stayed. Only a scope tells
			// the two apart (an external resource is declared with one), so ask for it.
			if res.NoAccess != "" && res.Scope == "" {
				errorList.
					WithObjectID("rbac.yaml/"+res.Key()).
					Warnf("%s is denied access but the module ships no CRD for it and the entry names no scope; if the resource is external, add scope: Namespaced|Cluster to say so, if its CRD was removed, drop the entry",
						res.Key())
			}

			continue
		}

		if _, resourceKnown := known[res.Key()]; !resourceKnown {
			errorList.
				WithObjectID("rbac.yaml/"+res.Key()).
				Warnf("%s names a resource the module's CRDs of group %s do not have; check the spelling, or drop the entry if the resource is gone",
					res.Key(), res.Group)
		}
	}
}

// appendStubFix returns the autofix for a CRD without an entry: append an undecided stub to
// rbac.yaml. The closure reads the file when it runs, so several stubs written in one run land
// in the same file; it leaves an already present entry alone, so the fix is idempotent. It
// returns an error on purpose after a successful write: the stub is not a decision, and the
// finding must stay in the output and in the exit code of the run that wrote it (R33).
func appendStubFix(modulePath string, crd crdInfo) errors.AutofixFunc {
	path := rbacyaml.Path(modulePath)

	return func() error {
		// One stub per CRD per run, however many render variants reported it (R36).
		return fixOnce(path+"#"+crd.Key(), func() error {
			added, err := appendStub(path, crd.Group, crd.Plural)
			if err != nil {
				return fmt.Errorf("add a stub for %s to %s: %w", crd.Key(), rbacyaml.Filename, err)
			}

			if !added {
				return nil
			}

			return fmt.Errorf("a stub for %s was added to %s; decide its access (noAccess: %q is not a decision)",
				crd.Key(), rbacyaml.Filename, rbacyaml.NoAccessTODO)
		})
	}
}

// appendStub adds `- group: <group>\n  resource: <resource>\n  noAccess: "TODO"` to the
// resources of the declaration, keeping the rest of the file -- comments included -- as it is.
// It reports whether anything was written.
func appendStub(path, group, resource string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false, err
	}

	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return false, stderrors.New("the file is not a YAML mapping")
	}

	doc := root.Content[0]
	resources := mappingValue(doc, "resources")

	if resources == nil {
		doc.Content = append(doc.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "resources"},
			&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
		resources = doc.Content[len(doc.Content)-1]
	}

	if resources.Kind != yaml.SequenceNode {
		// `resources: null` or a scalar: replace with a sequence so the stub has somewhere to go.
		*resources = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}

	for _, item := range resources.Content {
		if scalarValue(mappingValue(item, "group")) == group && scalarValue(mappingValue(item, "resource")) == resource {
			return false, nil
		}
	}

	resources.Content = append(resources.Content, &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "group"}, {Kind: yaml.ScalarNode, Value: group},
			{Kind: yaml.ScalarNode, Value: "resource"}, {Kind: yaml.ScalarNode, Value: resource},
			{Kind: yaml.ScalarNode, Value: "noAccess"}, {Kind: yaml.ScalarNode, Value: rbacyaml.NoAccessTODO, Style: yaml.DoubleQuotedStyle},
		},
	})

	var buf bytes.Buffer

	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(&root); err != nil {
		return false, err
	}

	if err := encoder.Close(); err != nil {
		return false, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return true, writeFileAtomic(path, buf.Bytes(), info.Mode().Perm())
}

// mappingValue returns the value node of key in a mapping node, or nil.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

func scalarValue(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}

	return node.Value
}
