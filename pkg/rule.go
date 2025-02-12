package pkg

import (
	"github.com/deckhouse/dmt/internal/storage"
	corev1 "k8s.io/api/core/v1"
)

type RuleMeta struct {
	Name string
}

func (m *RuleMeta) GetName() string {
	return m.Name
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

type ContainerRule struct {
	ExcludeRules []ContainerRuleExclude
}

func (r *ContainerRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.ExcludeRules {
		return rule.Enabled(object, container)
	}

	return true
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
