/*
Copyright 2025 Flant JSC

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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PriorityClassRuleName = "object-priority-class"
)

func NewPriorityClassRule() *PriorityClassRule {
	return &PriorityClassRule{
		RuleMeta: pkg.RuleMeta{
			Name: PriorityClassRuleName,
		},
	}
}

type PriorityClassRule struct {
	pkg.RuleMeta
}

func (r *PriorityClassRule) ObjectPriorityClass(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !isPriorityClassSupportedKind(object.Unstructured.GetKind()) {
		return
	}

	priorityClass, err := getPriorityClass(object)
	if err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	validatePriorityClass(priorityClass, object, errorList)
}

func isPriorityClassSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet":
		return true
	default:
		return false
	}
}

func getPriorityClass(object storage.StoreObject) (string, error) {
	converter := runtime.DefaultUnstructuredConverter

	var priorityClass string
	var err error

	switch object.Unstructured.GetKind() {
	case "Deployment":
		deployment := new(appsv1.Deployment)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		priorityClass = deployment.Spec.Template.Spec.PriorityClassName
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		priorityClass = daemonset.Spec.Template.Spec.PriorityClassName
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		priorityClass = statefulset.Spec.Template.Spec.PriorityClassName
	}

	return priorityClass, err
}

func validatePriorityClass(priorityClass string, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	switch priorityClass {
	case "":
		errorList.WithObjectID(object.Identity()).WithValue(priorityClass).
			Error("Priority class must not be empty")
	case "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low", "cluster-critical":
	default:
		errorList.WithObjectID(object.Identity()).WithValue(priorityClass).
			Error("Priority class is not allowed")
	}
}
