package pkg

import (
	"github.com/deckhouse/dmt/internal/storage"
	corev1 "k8s.io/api/core/v1"
)

type KindRuleExclude struct {
	Kind string
	Name string
}

func (e *KindRuleExclude) Enabled(object storage.StoreObject, container *corev1.Container) bool {
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
