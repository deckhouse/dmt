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
	SecurityPolicyExceptionSchemaRuleName      = "security-policy-exception-schema"

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

		templates, err := pss.SPEAllowancePaths()
		if err != nil {
			errorList.Errorf("Cannot check SecurityPolicyException allowances: %v", err)

			continue
		}

		spec, _ := o.Unstructured.Object["spec"].(map[string]any)

		for _, path := range undescribedAllowances(spec, templates) {
			errorList.WithValue(path).Errorf("SecurityPolicyException %s/%s: allowance %s has no metadata.description",
				o.Unstructured.GetNamespace(), o.Unstructured.GetName(), path)
		}
	}
}

// undescribedAllowances returns the paths of the allowances present in spec without a
// description. templates are allowance paths from the CRD schema (pss.SPEAllowancePaths),
// pss.ArrayItem segments expand to every element of the array.
func undescribedAllowances(spec map[string]any, templates [][]string) []string {
	var res []string

	var walk func(path string, node any, tpl []string)

	walk = func(path string, node any, tpl []string) {
		switch {
		case len(tpl) == 0:
			if !hasDescription(node) {
				res = append(res, path)
			}
		case tpl[0] == pss.ArrayItem:
			list, _ := node.([]any)
			for i, item := range list {
				walk(fmt.Sprintf("%s[%d]", path, i), item, tpl[1:])
			}
		default:
			m, _ := node.(map[string]any)
			if child := m[tpl[0]]; child != nil {
				walk(path+"."+tpl[0], child, tpl[1:])
			}
		}
	}

	for _, tpl := range templates {
		walk("spec", spec, tpl)
	}

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

func NewSecurityPolicyExceptionSchemaRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *SecurityPolicyExceptionSchemaRule {
	return &SecurityPolicyExceptionSchemaRule{
		RuleMeta:  pkg.RuleMeta{Name: SecurityPolicyExceptionSchemaRuleName},
		module:    m,
		errorList: errorList.WithRule(SecurityPolicyExceptionSchemaRuleName),
	}
}

// SecurityPolicyExceptionSchemaRule validates SecurityPolicyExceptions against the
// openAPIV3Schema of the CRD, as the apiserver does: the PSS rego reads whatever it
// is given and may accept an SPE the cluster rejects. pod-security-standards still
// sees such an SPE.
type SecurityPolicyExceptionSchemaRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*SecurityPolicyExceptionSchemaRule)(nil)

func (r *SecurityPolicyExceptionSchemaRule) Check(_ context.Context) {
	for _, o := range r.module.GetStorage() {
		if !isSPE(o) {
			continue
		}

		errorList := r.errorList.WithFilePath(o.GetPath()).WithObjectID(o.Identity())

		problems, err := pss.ValidateSPE(o.Unstructured.Object)
		if err != nil {
			errorList.Errorf("Cannot validate SecurityPolicyException: %v", err)

			continue
		}

		for _, p := range problems {
			errorList.Errorf("SecurityPolicyException %s/%s does not match the CRD schema: %s",
				o.Unstructured.GetNamespace(), o.Unstructured.GetName(), p)
		}
	}
}
