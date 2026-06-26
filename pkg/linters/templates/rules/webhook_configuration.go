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
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	WebhookConfigurationRuleName = "webhook-configuration-annotations"

	AnnotationWeight           = "werf.io/weight"
	AnnotationDeployDependency = "werf.io/deploy-dependency"
)

func NewWebhookConfigurationRule(excludeRules []pkg.KindRuleExclude) *WebhookConfigurationRule {
	return &WebhookConfigurationRule{
		RuleMeta: pkg.RuleMeta{
			Name: WebhookConfigurationRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

type WebhookConfigurationRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

func (r *WebhookConfigurationRule) ValidateWebhookConfigurationAnnotations(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	for _, object := range m.GetStorage() {
		kind := object.Unstructured.GetKind()
		if kind != "ValidatingWebhookConfiguration" && kind != "MutatingWebhookConfiguration" {
			continue
		}

		if !r.Enabled(kind, object.Unstructured.GetName()) {
			continue
		}

		annotations := object.Unstructured.GetAnnotations()

		_, hasWeight := annotations[AnnotationWeight]
		_, hasDeployDependency := annotations[AnnotationDeployDependency]

		if !hasWeight && !hasDeployDependency {
			errorList.WithObjectID(object.Identity()).
				WithFilePath(object.GetPath()).
				Errorf("%s %q must have either %q or %q annotation", kind, object.Unstructured.GetName(), AnnotationWeight, AnnotationDeployDependency)
		}
	}
}
