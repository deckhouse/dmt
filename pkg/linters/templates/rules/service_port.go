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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

const (
	ServicePortRuleName = "service-port"
)

func NewServicePortRule(excludeRules []pkg.ServicePortExclude) *ServicePortRule {
	return &ServicePortRule{
		RuleMeta: pkg.RuleMeta{
			Name: ServicePortRuleName,
		},
		ServicePortRule: pkg.ServicePortRule{
			ExcludeRules: excludeRules,
		},
	}
}

func NewServicePortRuleTracked(trackedRule *exclusions.TrackedServicePortRule) *ServicePortRuleTracked {
	return &ServicePortRuleTracked{
		RuleMeta: pkg.RuleMeta{
			Name: ServicePortRuleName,
		},
		ServicePortRule: trackedRule.ServicePortRule,
		trackedRule:     trackedRule,
	}
}

type ServicePortRule struct {
	pkg.RuleMeta
	pkg.ServicePortRule
}

type ServicePortRuleTracked struct {
	pkg.RuleMeta
	pkg.ServicePortRule
	trackedRule *exclusions.TrackedServicePortRule
}

func (r *ServicePortRuleTracked) ObjectServiceTargetPort(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

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
		// Use tracked rule to check if service/port should be excluded and mark exclusions as used
		if !r.trackedRule.Enabled(service.GetName(), port.Name) {
			// TODO: add metrics
			continue
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

func (r *ServicePortRule) ObjectServiceTargetPort(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

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
			continue
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
