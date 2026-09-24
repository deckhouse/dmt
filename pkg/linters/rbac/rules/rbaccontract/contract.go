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

// Package rbaccontract holds the constants of the Deckhouse RBACv2 role model that the rbac
// linter rules and the rbac.yaml generator share: lineages, the levels each lineage accepts,
// the legacy access levels of user-authz v1, the label and annotation keys, and the
// conventional localized texts of view/edit capabilities.
//
// Every constant names its source in the deckhouse repository. The contract is described for
// humans in modules/140-user-authz/docs/internal/RBACV2_MODULE_MIGRATION.md and enforced
// in-tree by testing/rbacv2/rbacv2_templates_validation_test.go.
package rbaccontract

import (
	"slices"
	"strings"
)

// Label and annotation keys of the role model
// (modules/140-user-authz/docs/internal/RBACV2_MODULE_MIGRATION.md, "Reference of role labels and annotations").
const (
	LabelKind        = "rbac.deckhouse.io/kind"
	LabelScope       = "rbac.deckhouse.io/scope"
	LabelCapability  = "rbac.deckhouse.io/capability"
	LabelUseRole     = "rbac.deckhouse.io/use-role"
	LabelDelegatable = "rbac.deckhouse.io/delegatable"
	LabelNamespace   = "rbac.deckhouse.io/namespace"
	LabelModule      = "module"

	// KindLegacyUse and KindLegacyManage are the kinds of the RBACv2 scheme before the DKP 1.78 role
	// model: d8:use:capability:module:<m>:<action> aggregated into aggregate-to-kubernetes-as, and
	// d8:manage:permission:module:<m>:<action> aggregated into a subsystem. An external module may
	// still ship them, alone or beside the new objects behind the version gate.
	KindLegacyUse    = "use"
	KindLegacyManage = "manage"

	// GateMarker is the helper rbacv2-migrate-module.sh defines when it keeps both schemes in one
	// template: `include "<module>.rbacv2_new_scheme"` answers which one the render is for from
	// global.deckhouseVersion. A template that carries it renders exactly one of the two.
	GateMarker    = "rbacv2_new_scheme"
	LabelHeritage = "heritage"

	// AggregationLabelPrefix and AggregationLabelSuffix frame the lineage in
	// rbac.deckhouse.io/aggregate-to-<lineage>-as.
	AggregationLabelPrefix = "rbac.deckhouse.io/aggregate-to-"
	AggregationLabelSuffix = "-as"

	// AccessLevelAnnotation marks a legacy (user-authz v1) ClusterRole with its access level
	// (modules/140-user-authz/hooks/... and templates/user-authz-cluster-roles.yaml of every module).
	AccessLevelAnnotation = "user-authz.deckhouse.io/access-level"

	KindRole       = "role"
	KindCapability = "capability"

	// Capability name prefixes per scope (RBACV2_MODULE_MIGRATION.md, "Naming").
	NamespaceCapabilityPrefix = "d8:namespace-capability:"
	SystemCapabilityPrefix    = "d8:system-capability:"
	LegacyRolePrefix          = "d8:user-authz:"
)

// Lineages of the role model. A capability aggregates into the roles of one or more lineages
// through the aggregate-to-<lineage>-as label.
const (
	LineageNamespace = "namespace"
	LineageProject   = "project"
	LineageSystem    = "system"
)

// Subsystems are the lineages of the subsystem roles d8:subsystem:<name>:<level>
// (modules/140-user-authz/templates/rbacv2/global/subsystem/roles/<name>/).
var Subsystems = []string{
	"deckhouse",
	"infrastructure",
	"kubernetes",
	"networking",
	"observability",
	"security",
	"storage",
}

// Levels a capability may aggregate to, per lineage. The namespace lineage carries the full
// ladder; the system and subsystem lineages have no user and admin rungs
// (RBACV2_MODULE_MIGRATION.md, "Access levels"; spec 005 R29).
var (
	NamespaceLevels = []string{"viewer", "user", "manager", "admin", "superadmin"}
	SystemLevels    = []string{"viewer", "manager", "superadmin"}
	// ProjectLevels are the levels of the project lineage; a module capability never aggregates
	// there directly (project roles aggregate namespace roles), but the contract check on
	// platform roles needs the set.
	ProjectLevels = slices.Clone(NamespaceLevels)
)

// ContractVersion is the version of the platform contract the generator writes templates for. It is
// recorded in the header of every generated file, so that a file produced under an older contract
// is recognizable after the contract changes. Bump it when the generated shape changes.
const ContractVersion = "2"

// LegacyKebab returns the name suffix of the legacy ClusterRole for an access level, as the
// modules spell it today (d8:user-authz:<module>:cluster-editor for ClusterEditor).
func LegacyKebab(level string) string {
	var b []byte

	for i := 0; i < len(level); i++ {
		c := level[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				b = append(b, '-')
			}

			c += 'a' - 'A'
		}

		b = append(b, c)
	}

	return string(b)
}

// LegacyLevels is the access-level enum of ClusterAuthorizationRule
// (modules/140-user-authz/crds/clusterauthorizationrule.yaml); AuthorizationRule serves only the
// first four.
var LegacyLevels = []string{"User", "PrivilegedUser", "Editor", "Admin", "ClusterEditor", "ClusterAdmin", "SuperAdmin"}

// Verbs are the resource verbs Kubernetes RBAC knows, with the wildcard. rbac.yaml lists verbs
// explicitly and has no aliases (spec 005 R2); the wildcard is refused at every user-facing level
// (rbacyaml.Validate) and in a rendered capability (the contract rule), and judged by the wildcards
// rule in a ServiceAccount's own rules.
var Verbs = append(slices.Clone(ResourceVerbs), "*")

// ResourceVerbs are the verbs a rule may list, without the wildcard.
var ResourceVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}

// AllLineages returns every lineage a capability label may name: the three base lineages and
// the seven subsystems.
func AllLineages() []string {
	out := make([]string, 0, 3+len(Subsystems))
	out = append(out, LineageNamespace, LineageProject, LineageSystem)
	out = append(out, Subsystems...)

	return out
}

// LevelsOf returns the levels the given lineage accepts, or nil for an unknown lineage.
func LevelsOf(lineage string) []string {
	switch lineage {
	case LineageNamespace:
		return NamespaceLevels
	case LineageProject:
		return ProjectLevels
	case LineageSystem:
		return SystemLevels
	}

	for _, s := range Subsystems {
		if s == lineage {
			return SystemLevels
		}
	}

	return nil
}

// IsSubsystem reports whether the name is one of the seven subsystems.
func IsSubsystem(name string) bool {
	return slices.Contains(Subsystems, name)
}

// CapabilityAction maps a level to the action suffix of the capability it produces:
// viewer -> view, manager -> edit, the rest as they are (ADR "Что генерируется"; the live
// convention is use/admin.yaml with marker namespace-capability.cert-manager.admin).
func CapabilityAction(level string) string {
	switch level {
	case "viewer":
		return "view"
	case "manager":
		return "edit"
	}

	return level
}

// LevelOfAction is the inverse of CapabilityAction: view -> viewer, edit -> manager, the rest as
// they are.
func LevelOfAction(action string) string {
	switch action {
	case "view":
		return "viewer"
	case "edit":
		return "manager"
	}

	return action
}

// BindingSuffix turns a role name into the last segment of the binding the generator names after
// it: the d8: prefix goes, the colons become dashes (d8:rbac-proxy -> rbac-proxy,
// system:auth-delegator -> system-auth-delegator).
func BindingSuffix(roleName string) string {
	return strings.ReplaceAll(strings.TrimPrefix(roleName, "d8:"), ":", "-")
}

// ConventionalActions are the capability actions whose localized texts come from the platform
// convention and need no capabilities entry in rbac.yaml.
var ConventionalActions = []string{"view", "edit"}

// IsConventionalAction reports whether the texts of a capability with this action are supplied
// by the platform (view/edit) rather than by the declaration.
func IsConventionalAction(action string) bool {
	return action == "view" || action == "edit"
}

// Text is a localized title/description pair.
type Text struct {
	EN string
	RU string
}

// ConventionalTexts are the titles and descriptions of view/edit capabilities of both lineages,
// with %s standing for the module name. Source: the TEXTS table of
// modules/140-user-authz/docs/internal/rbacv2-migrate-module.sh, reproduced in the ADR.
var ConventionalTexts = map[string]struct{ Title, Description Text }{
	LineageNamespace + ".view": {
		Title:       Text{EN: "Module %s: view", RU: "Модуль %s: просмотр"},
		Description: Text{EN: "Read-only access to %s resources in a namespace.", RU: "Доступ только на чтение к ресурсам модуля %s в пространстве имён."},
	},
	LineageNamespace + ".edit": {
		Title:       Text{EN: "Module %s: edit", RU: "Модуль %s: редактирование"},
		Description: Text{EN: "Manage %s resources in a namespace.", RU: "Управление ресурсами модуля %s в пространстве имён."},
	},
	LineageSystem + ".view": {
		Title:       Text{EN: "Module %s: view configuration", RU: "Модуль %s: просмотр конфигурации"},
		Description: Text{EN: "Read-only access to the %s module configuration.", RU: "Доступ только на чтение к конфигурации модуля %s."},
	},
	LineageSystem + ".edit": {
		Title:       Text{EN: "Module %s: edit configuration", RU: "Модуль %s: управление конфигурацией"},
		Description: Text{EN: "Manage the %s module configuration.", RU: "Управление конфигурацией модуля %s."},
	},
}

// Annotation keys of the localized texts.
const (
	AnnotationTitleEN       = "en.meta.deckhouse.io/title"
	AnnotationTitleRU       = "ru.meta.deckhouse.io/title"
	AnnotationDescriptionEN = "en.meta.deckhouse.io/description"
	AnnotationDescriptionRU = "ru.meta.deckhouse.io/description"
)

// I18nAnnotations lists the four annotations every RBACv2 role and capability must carry.
var I18nAnnotations = []string{AnnotationTitleEN, AnnotationTitleRU, AnnotationDescriptionEN, AnnotationDescriptionRU}

// IsLegacyKind reports whether the kind label names the manage/use scheme that preceded the 1.78
// role model.
func IsLegacyKind(kind string) bool {
	return kind == KindLegacyUse || kind == KindLegacyManage
}

// DeckhouseNamespaces are the namespaces the placement rule treats as the platform's own: there an
// account of templates/<dir>/ may carry the module name in front of the directory.
var DeckhouseNamespaces = []string{"d8-monitoring", "d8-system", "d8-admission-policy-engine", "d8-operator-trivy", "d8-log-shipper", "d8-local-path-provisioner"}

// IsDeckhouseNamespace reports whether the namespace is one of DeckhouseNamespaces.
func IsDeckhouseNamespace(ns string) bool {
	for _, n := range DeckhouseNamespaces {
		if n == ns {
			return true
		}
	}

	return false
}
