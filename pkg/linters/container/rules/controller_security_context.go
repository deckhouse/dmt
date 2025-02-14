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
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ControllerSecurityContextRuleName = "controller-security-context"
)

func NewControllerSecurityContextRule(excludeRules []pkg.KindRuleExclude) *ControllerSecurityContextRule {
	return &ControllerSecurityContextRule{
		RuleMeta: pkg.RuleMeta{
			Name: ControllerSecurityContextRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

type ControllerSecurityContextRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

func (r *ControllerSecurityContextRule) ControllerSecurityContext(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !isSecurityContextSupportedKind(object.Unstructured.GetKind()) {
		return
	}

	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("GetPodSecurityContext failed: %v", err)

		return
	}

	if securityContext == nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Object's SecurityContext is not defined")

		return
	}

	checkSecurityContextParameters(securityContext, object, errorList)
}

func isSecurityContextSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
		return true
	default:
		return false
	}
}

func checkSecurityContextParameters(securityContext *corev1.PodSecurityContext, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if securityContext.RunAsNonRoot == nil {
		errorList.WithObjectID(object.Identity()).
			Error("Object's SecurityContext missing parameter RunAsNonRoot")
	}

	if securityContext.RunAsUser == nil {
		errorList.WithObjectID(object.Identity()).
			Error("Object's SecurityContext missing parameter RunAsUser")
	}

	if securityContext.RunAsGroup == nil {
		errorList.WithObjectID(object.Identity()).
			Error("Object's SecurityContext missing parameter RunAsGroup")
	}

	if securityContext.RunAsNonRoot != nil && securityContext.RunAsUser != nil && securityContext.RunAsGroup != nil {
		checkRunAsNonRoot(securityContext, object, errorList)
	}
}

func checkRunAsNonRoot(securityContext *corev1.PodSecurityContext, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	value := fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup)

	switch *securityContext.RunAsNonRoot {
	case true:
		if (*securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534) &&
			(*securityContext.RunAsUser != 64535 || *securityContext.RunAsGroup != 64535) {
			errorList.WithObjectID(object.Identity()).WithValue(value).
				Error("Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)")
		}
	case false:
		if *securityContext.RunAsUser != 0 || *securityContext.RunAsGroup != 0 {
			errorList.WithObjectID(object.Identity()).WithValue(value).
				Error("Object's SecurityContext has `RunAsNonRoot: false`, but RunAsUser:RunAsGroup differs from 0:0")
		}
	}
}
