package pkg

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
)

type RuleMeta struct {
	Name string
}

func (m *RuleMeta) GetName() string {
	return m.Name
}

type BoolRule struct {
	Exclude bool
}

func (r *BoolRule) Enabled() bool {
	return !r.Exclude
}

type StringRule struct {
	ExcludeRules []StringRuleExclude
}

func (r *StringRule) Enabled(str string) bool {
	for _, rule := range r.ExcludeRules {
		return rule.Enabled(str)
	}

	return true
}

type KindRule struct {
	ExcludeRules []KindRuleExclude
}

func (r *KindRule) Enabled(object storage.StoreObject) bool {
	for _, rule := range r.ExcludeRules {
		return rule.Enabled(object)
	}

	return true
}

type TargetRefRule struct {
	ExcludeRules []TargetRefRuleExclude
}

func (r *TargetRefRule) Enabled(kind, name string) bool {
	for _, rule := range r.ExcludeRules {
		return rule.Enabled(kind, name)
	}

	return true
}

type ContainerRule struct {
	ExcludeRules []ContainerRuleExclude
}

func (r *ContainerRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.ExcludeRules {
		return rule.Enabled(object, container)
	}

	return true
}

type StringRuleExclude string

func (e StringRuleExclude) Enabled(str string) bool {
	return string(e) != str
}

type KindRuleExclude struct {
	Kind string
	Name string
}

func (e *KindRuleExclude) Enabled(object storage.StoreObject) bool {
	if e.Kind == object.Unstructured.GetKind() &&
		e.Name == object.Unstructured.GetName() {
		return false
	}

	return true
}

type TargetRefRuleExclude struct {
	Kind string
	Name string
}

func (e *TargetRefRuleExclude) Enabled(kind, name string) bool {
	if e.Kind == kind &&
		e.Name == name {
		return false
	}

	return true
}

type ContainerRuleExclude struct {
	Kind      string
	Name      string
	Container string
}

func (e *ContainerRuleExclude) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	if e.Kind == object.Unstructured.GetKind() &&
		e.Name == object.Unstructured.GetName() &&
		(e.Container == "" || e.Container == container.Name) {
		return false
	}

	return true
}
