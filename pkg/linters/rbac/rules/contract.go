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
	"regexp"
	"slices"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

const (
	ContractRuleName = "contract"

	// rbacv2TemplatesDir is the directory whose rendered ClusterRoles the contract applies to --
	// the same population the platform test in deckhouse/testing/rbacv2 walks. The compatibility
	// aliases in templates/rbacv2-compat/ keep the pre-1.78 names on purpose and are outside it.
	rbacv2TemplatesDir = "templates/rbacv2/"

	// dictRoleName is a standalone helper role bound by the handle_dict_bindings hook; it lives
	// outside the role/capability framework and carries no kind/scope labels.
	dictRoleName = "d8:dict"
)

var (
	aggregateLabelRe = regexp.MustCompile(`^rbac\.deckhouse\.io/aggregate-to-([a-z0-9-]+)-as$`)
	// labelValueRe is the Kubernetes label-value grammar the capability marker must satisfy.
	labelValueRe = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`)

	roleNameRe = map[string]*regexp.Regexp{
		"system":    regexp.MustCompile(`^d8:system:([a-z]+)$`),
		"subsystem": regexp.MustCompile(`^d8:subsystem:([a-z0-9-]+):([a-z]+)$`),
		"namespace": regexp.MustCompile(`^d8:namespace:([a-z]+)$`),
		"project":   regexp.MustCompile(`^d8:project:([a-z]+)$`),
	}

	capabilityNamePrefix = map[string]string{
		"system":    "d8:system-capability:",
		"subsystem": "d8:subsystem-capability:",
		"namespace": "d8:namespace-capability:",
		"project":   "d8:project-capability:",
	}

	validScopes = []string{"system", "subsystem", "namespace", "project"}
)

// ContractRule checks the rendered RBACv2 ClusterRoles of a module against the platform's label
// and naming contract -- the first part of deckhouse/testing/rbacv2/rbacv2_templates_validation_test.go,
// so that a module outside the platform repository is held to the same contract. It works on
// rendered objects, not template text, and needs no rbac.yaml.
//
// One check is new here: a cluster-scoped resource inside a namespace capability. Such a rule
// grants nothing through the RoleBinding the capability is bound with. It is reported as a
// warning: three in-tree modules carry such rules today, and it becomes an error once they are
// fixed. The scope of a resource is known from the module's CRDs or from its rbac.yaml entry;
// a resource the run knows nothing about is not judged.
//
// What stays in the platform test on purpose: the levels of sensitive capabilities and the
// closure of aggregation across two modules (rbacv2_capability_levels_test.go), and the global
// uniqueness of the capability marker -- a rule sees one module.
type ContractRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*ContractRule)(nil)

func NewContractRule(excludeRules []pkg.KindRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *ContractRule {
	return &ContractRule{
		RuleMeta:  pkg.RuleMeta{Name: ContractRuleName},
		KindRule:  pkg.KindRule{ExcludeRules: excludeRules},
		module:    m,
		errorList: errorList.WithRule(ContractRuleName),
	}
}

func (r *ContractRule) Check(_ context.Context) {
	scopes := r.resourceScopes()

	// Sorted for a deterministic order of findings across runs and render variants.
	objects := make([]storage.StoreObject, 0)

	for _, object := range r.module.GetStorage() {
		if object.Unstructured.GetKind() != "ClusterRole" || !strings.HasPrefix(object.ShortPath(), rbacv2TemplatesDir) {
			continue
		}

		if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
			continue
		}

		objects = append(objects, object)
	}

	sort.Slice(objects, func(i, j int) bool { return objects[i].Unstructured.GetName() < objects[j].Unstructured.GetName() })

	for _, object := range objects {
		errorList := r.errorList.WithObjectID(object.Identity()).WithFilePath(object.ShortPath())

		role := new(rbacv1.ClusterRole)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Unstructured.UnstructuredContent(), role); err != nil {
			errorList.Errorf("cannot convert the object to a ClusterRole: %v", err)
			continue
		}

		// An object of the scheme before 1.78 is not held to the contract check by check -- every
		// one of them would fail, and the finding that matters is "migrate". A module that serves
		// both models renders the legacy object only when the values say the cluster is below 1.78
		// (rbacv2-migrate-module.sh gates the template), and that is not a finding at all.
		if kind := role.Labels[rbaccontract.LabelKind]; rbaccontract.IsLegacyKind(kind) {
			if !templateHasGate(r.module.GetPath(), object.ShortPath()) {
				errorList.Errorf("ClusterRole %q is of the legacy RBACv2 scheme (%s: %s, the manage/use model before DKP 1.78); migrate the module with rbacv2-migrate-module.sh from modules/140-user-authz/docs/internal/ of the deckhouse repository, or describe it in %s and run `%s`",
					role.Name, rbaccontract.LabelKind, kind, rbacyaml.Filename, FixCommand)
			}

			continue
		}

		checkContract(role, r.module.GetName(), scopes, errorList)
	}
}

// resourceScopes collects what this run knows about resource scopes: the module's CRDs and the
// entries of its rbac.yaml. A resource missing from the map is not judged.
func (r *ContractRule) resourceScopes() rbacyaml.CRDScopes {
	scopes := make(rbacyaml.CRDScopes)

	if crds, err := moduleCRDs(r.module.GetPath()); err == nil {
		for _, crd := range crds {
			scopes[crd.Key()] = crd.Scope
		}
	}

	if decl, err := rbacyaml.Load(r.module.GetPath()); err == nil {
		for _, res := range decl.Resources {
			if res.Scope != "" && !res.IsWildcard() && !res.IsSubresource() {
				if _, fromCRD := scopes[res.Key()]; !fromCRD {
					scopes[res.Key()] = res.Scope
				}
			}
		}
	}

	return scopes
}

// checkContract applies the contract to one rendered ClusterRole. The checks and their messages
// follow the platform test so that both give the same verdict on a module.
func checkContract(role *rbacv1.ClusterRole, module string, scopes rbacyaml.CRDScopes, errorList *errors.LintRuleErrorsList) {
	name := role.Name
	labels := role.Labels
	annotations := role.Annotations

	if !strings.HasPrefix(name, "d8:") {
		errorList.Errorf("name %q must start with the d8: prefix", name)
	}

	// The platform test cannot know which module a role or capability belongs to; dmt does (spec
	// 005 R21). Helpers outside the framework (no kind label, such as d8:dict) are not judged.
	if got, framework := labels[rbaccontract.LabelModule], labels[rbaccontract.LabelKind] != ""; framework && got != module {
		errorList.Errorf("label %s must be the module name %q, got %q", rbaccontract.LabelModule, module, got)
	}

	for _, key := range rbaccontract.I18nAnnotations {
		if annotations[key] == "" {
			errorList.Errorf("missing the %s annotation: every RBACv2 role and capability carries localized en/ru title and description", key)
		}
	}

	if name == dictRoleName {
		return
	}

	kind := labels[rbaccontract.LabelKind]
	scope := labels[rbaccontract.LabelScope]

	if kind != rbaccontract.KindRole && kind != rbaccontract.KindCapability {
		errorList.Errorf("label %s must be %q or %q, got %q", rbaccontract.LabelKind, rbaccontract.KindRole, rbaccontract.KindCapability, kind)
		return
	}

	if !slices.Contains(validScopes, scope) {
		errorList.Errorf("label %s must be one of %s, got %q", rbaccontract.LabelScope, strings.Join(validScopes, "/"), scope)
		return
	}

	switch kind {
	case rbaccontract.KindRole:
		checkRole(role, scope, errorList)
	case rbaccontract.KindCapability:
		checkCapability(role, scope, scopes, errorList)
	}

	// Aggregation labels: the lineage must exist and the level must be one of that lineage (R29).
	for _, key := range sortedKeys(labels) {
		m := aggregateLabelRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}

		lineage, level := m[1], labels[key]

		levels := rbaccontract.LevelsOf(lineage)
		if levels == nil {
			errorList.Errorf("aggregation label %q targets unknown lineage %q", key, lineage)
			continue
		}

		if !slices.Contains(levels, level) {
			errorList.Errorf("aggregation label %q has invalid level %q; the %s lineage has %s", key, level, lineage, strings.Join(levels, ", "))
		}
	}

	if _, ok := labels[rbaccontract.LabelDelegatable]; ok {
		if kind != rbaccontract.KindRole || (scope != "namespace" && scope != "project") {
			errorList.Errorf("label %s is only allowed on namespace/project roles", rbaccontract.LabelDelegatable)
		}
	}
}

func checkRole(role *rbacv1.ClusterRole, scope string, errorList *errors.LintRuleErrorsList) {
	name, labels := role.Name, role.Labels

	re := roleNameRe[scope]

	m := re.FindStringSubmatch(name)
	if m == nil {
		errorList.Errorf("role name %q does not match the %s-scope pattern %s", name, scope, re)
		return
	}

	level := m[len(m)-1]
	if !slices.Contains(rbaccontract.NamespaceLevels, level) {
		errorList.Errorf("role name %q has invalid level %q", name, level)
	}

	if scope == "subsystem" {
		if !rbaccontract.IsSubsystem(m[1]) {
			errorList.Errorf("role name %q references unknown subsystem %q", name, m[1])
		}

		if got := labels["rbac.deckhouse.io/subsystem"]; got != m[1] {
			errorList.Errorf("label rbac.deckhouse.io/subsystem %q does not match the subsystem %q from the role name", got, m[1])
		}
	}

	if scope == "system" || scope == "subsystem" {
		if useRole := labels[rbaccontract.LabelUseRole]; !slices.Contains(rbaccontract.NamespaceLevels, useRole) {
			errorList.Errorf("label %s must carry a valid level, got %q", rbaccontract.LabelUseRole, useRole)
		}
	}

	if len(role.Rules) > 0 {
		errorList.Errorf("role %q must not define its own rules; move them into a capability", name)
	}

	if role.AggregationRule == nil || len(role.AggregationRule.ClusterRoleSelectors) == 0 {
		errorList.Errorf("role %q must define aggregationRule.clusterRoleSelectors", name)
		return
	}

	for _, selector := range role.AggregationRule.ClusterRoleSelectors {
		for _, key := range sortedKeys(selector.MatchLabels) {
			value := selector.MatchLabels[key]

			m := aggregateLabelRe.FindStringSubmatch(key)
			if m == nil {
				errorList.Errorf("role %q aggregation selector uses non-aggregation label %q", name, key)
				continue
			}

			levels := rbaccontract.LevelsOf(m[1])
			if levels == nil {
				errorList.Errorf("role %q aggregation selector targets unknown lineage %q", name, m[1])
			} else if !slices.Contains(levels, value) {
				errorList.Errorf("role %q aggregation selector has invalid level %q", name, value)
			}
		}
	}
}

func checkCapability(role *rbacv1.ClusterRole, scope string, scopes rbacyaml.CRDScopes, errorList *errors.LintRuleErrorsList) {
	name, labels := role.Name, role.Labels

	if prefix := capabilityNamePrefix[scope]; !strings.HasPrefix(name, prefix) {
		errorList.Errorf("capability name %q must start with %q for scope %q", name, prefix, scope)
	}

	if len(role.Rules) == 0 {
		errorList.Errorf("capability %q must define rules", name)
	}

	if role.AggregationRule != nil {
		errorList.Errorf("capability %q must not define aggregationRule", name)
	}

	var aggregates bool

	for key := range labels {
		if aggregateLabelRe.MatchString(key) {
			aggregates = true
			break
		}
	}

	if !aggregates {
		errorList.Errorf("capability %q does not aggregate into any role (no aggregate-to-*-as labels)", name)
	}

	marker := labels[rbaccontract.LabelCapability]

	switch {
	case marker == "":
		errorList.Errorf("capability %q must carry the %s label", name, rbaccontract.LabelCapability)
	case len(marker) > 63 || !labelValueRe.MatchString(marker):
		errorList.Errorf("capability %q has invalid %s label value %q", name, rbaccontract.LabelCapability, marker)
	}

	// A namespace capability is granted through a RoleBinding; a cluster-scoped resource in it
	// grants nothing. Warn for now (D8): three in-tree modules carry such rules.
	if scope == "namespace" {
		for _, rule := range role.Rules {
			for _, group := range rule.APIGroups {
				for _, resource := range rule.Resources {
					if scopes[group+"/"+resource] == rbacyaml.ScopeCluster {
						errorList.Warnf("capability %q grants %s/%s, a cluster-scoped resource, in a namespace capability: bound through a RoleBinding the rule grants nothing; move it to a system capability", name, group, resource)
					}
				}
			}
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
