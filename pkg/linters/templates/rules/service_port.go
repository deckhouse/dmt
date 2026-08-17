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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ServicePortRuleName = "service-port"
)

func NewServicePortRule(excludeRules []pkg.ServicePortExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *ServicePortRule {
	return &ServicePortRule{
		RuleMeta: pkg.RuleMeta{
			Name: ServicePortRuleName,
		},
		ServicePortRule: pkg.ServicePortRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(ServicePortRuleName),
	}
}

type ServicePortRule struct {
	pkg.RuleMeta
	pkg.ServicePortRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*ServicePortRule)(nil)

func (r *ServicePortRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		r.checkObject(object)
	}
}

func (r *ServicePortRule) checkObject(object storage.StoreObject) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	switch object.Unstructured.GetKind() {
	case "Service":
	default:
		return
	}

	converter := runtime.DefaultUnstructuredConverter

	service := new(corev1.Service)
	if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), service); err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	for _, port := range service.Spec.Ports {
		if !r.Enabled(service.GetName(), port.Name) {
			// TODO: add metrics
			return
		}

		if port.TargetPort.Type == intstr.Int {
			if port.TargetPort.IntVal == 0 {
				errorList.WithObjectID(object.Identity() + " ; port = " + port.Name).
					Error("Service port must use an explicit named (non-numeric) target port")

				continue
			}

			errorList.WithObjectID(object.Identity() + " ; port = " + port.Name).WithValue(port.TargetPort.IntVal).
				Error("Service port must use a named (non-numeric) target port")
		}
	}
}
