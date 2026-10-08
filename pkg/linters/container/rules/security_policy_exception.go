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
	"fmt"
	"sort"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/internal/pss"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	SecurityPolicyExceptionDescriptionRuleName = "security-policy-exception-description"
	SecurityPolicyExceptionUnusedRuleName      = "security-policy-exception-unused"

	speRefLabel             = "security.deckhouse.io/security-policy-exception"
	speContainerLabelPrefix = speRefLabel + ".container."
)

func isSPE(o storage.StoreObject) bool {
	return o.Unstructured.GetAPIVersion() == pss.SPEAPIVersion && o.Unstructured.GetKind() == pss.SPEKind
}

func NewSecurityPolicyExceptionDescriptionRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *SecurityPolicyExceptionDescriptionRule {
	return &SecurityPolicyExceptionDescriptionRule{
		RuleMeta:  pkg.RuleMeta{Name: SecurityPolicyExceptionDescriptionRuleName},
		module:    m,
		errorList: errorList.WithRule(SecurityPolicyExceptionDescriptionRuleName),
	}
}

// SecurityPolicyExceptionDescriptionRule requires metadata.description on every
// allowance of a SecurityPolicyException: the descriptions make up the documentation
// of component privileges for certification.
type SecurityPolicyExceptionDescriptionRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*SecurityPolicyExceptionDescriptionRule)(nil)

func (r *SecurityPolicyExceptionDescriptionRule) Check(_ context.Context) {
	for _, o := range r.module.GetStorage() {
		if !isSPE(o) {
			continue
		}

		errorList := r.errorList.WithFilePath(o.GetPath()).WithObjectID(o.Identity())
		spec, _ := o.Unstructured.Object["spec"].(map[string]any)

		for _, path := range undescribedAllowances(spec) {
			errorList.WithValue(path).Errorf("SecurityPolicyException %s/%s: allowance %s has no metadata.description",
				o.Unstructured.GetNamespace(), o.Unstructured.GetName(), path)
		}
	}
}

// undescribedAllowances returns the paths of the allowances in spec without a
// description. An allowance is any map holding allowedValue/allowedValues, wherever it
// is, except two places where the CRD puts metadata on array items instead:
//   - spec.volumes.hostPath: each allowedValues item ({path, readOnly, metadata}),
//     the node itself has no metadata in the schema (it would be pruned);
//   - spec.network.hostPorts: an array of {port, protocol, metadata} with no
//     allowedValue(s) at all.
func undescribedAllowances(spec map[string]any) []string {
	var res []string

	check := func(path string, node any) {
		if !hasDescription(node) {
			res = append(res, path)
		}
	}

	checkItems := func(path string, items any) {
		list, _ := items.([]any)
		for i, item := range list {
			check(fmt.Sprintf("%s[%d]", path, i), item)
		}
	}

	var walk func(path string, node map[string]any)

	walk = func(path string, node map[string]any) {
		if path == "spec.volumes.hostPath" {
			checkItems(path+".allowedValues", node["allowedValues"])

			return
		}

		_, one := node["allowedValue"]
		_, many := node["allowedValues"]

		if one || many {
			// Do not descend: values (sysctls, seLinuxOptions items) are not allowances.
			check(path, node)

			return
		}

		keys := make([]string, 0, len(node))
		for k := range node {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			p := path + "." + k
			if p == "spec.network.hostPorts" {
				checkItems(p, node[k])

				continue
			}

			if child, ok := node[k].(map[string]any); ok {
				walk(p, child)
			}
		}
	}

	walk("spec", spec)

	return res
}

// hasDescription reports whether node has a metadata.description written by a human:
// not empty, not whitespace, not the TODO placeholder `dmt lint --fix` generates.
func hasDescription(node any) bool {
	m, _ := node.(map[string]any)
	meta, _ := m["metadata"].(map[string]any)
	desc, _ := meta["description"].(string)
	desc = strings.TrimSpace(desc)

	return desc != "" && !strings.EqualFold(desc, "TODO")
}

func NewSecurityPolicyExceptionUnusedRule(excludeRules []pkg.StringRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *SecurityPolicyExceptionUnusedRule {
	return &SecurityPolicyExceptionUnusedRule{
		RuleMeta:   pkg.RuleMeta{Name: SecurityPolicyExceptionUnusedRuleName},
		StringRule: pkg.StringRule{ExcludeRules: excludeRules},
		module:     m,
		errorList:  errorList.WithRule(SecurityPolicyExceptionUnusedRuleName),
	}
}

// SecurityPolicyExceptionUnusedRule reports SecurityPolicyExceptions no rendered
// pod (template) of the same namespace refers to by label. Warning by default: a
// component behind a feature flag may not render with default values. SPEs of pods
// helm does not render (static pods, operator-created pods) are excluded by name and
// reported as ignored.
type SecurityPolicyExceptionUnusedRule struct {
	pkg.RuleMeta
	pkg.StringRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*SecurityPolicyExceptionUnusedRule)(nil)

func (r *SecurityPolicyExceptionUnusedRule) Check(_ context.Context) {
	objects := r.module.GetStorage()

	// namespace -> SPE names referenced there
	used := map[string]map[string]bool{}

	for _, o := range objects {
		pod := podOf(o)
		if pod == nil {
			continue
		}

		ns := o.Unstructured.GetNamespace()

		for k, v := range nested(pod, "metadata", "labels") {
			name, _ := v.(string)
			if k != speRefLabel && !strings.HasPrefix(k, speContainerLabelPrefix) || name == "" {
				continue
			}

			if used[ns] == nil {
				used[ns] = map[string]bool{}
			}

			used[ns][name] = true
		}
	}

	for _, o := range objects {
		if !isSPE(o) || used[o.Unstructured.GetNamespace()][o.Unstructured.GetName()] {
			continue
		}

		errorList := r.errorList.WithFilePath(o.GetPath()).WithObjectID(o.Identity())
		if !r.Enabled(o.Unstructured.GetName()) {
			errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
		}

		errorList.Errorf("SecurityPolicyException %s/%s is not referenced by any rendered pod template of its namespace "+
			"(label %s or %s<container>)",
			o.Unstructured.GetNamespace(), o.Unstructured.GetName(), speRefLabel, speContainerLabelPrefix)
	}
}
