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
	"log/slog"
	"strings"

	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	IngressRuleName                = "ingress-rules"
	nginxAnnotationPrefix          = "nginx.ingress.kubernetes.io/"
	configurationSnippetAnnotation = nginxAnnotationPrefix + "configuration-snippet"
	ingressNginxHSTSAnnotation     = nginxAnnotationPrefix + "ingress-nginx-hsts"
	legacyHSTSDirective            = "add_header Strict-Transport-Security"
)

var unsafeIngressAnnotations = []string{
	configurationSnippetAnnotation,
	nginxAnnotationPrefix + "server-snippet",
	nginxAnnotationPrefix + "auth-snippet",
	nginxAnnotationPrefix + "modsecurity-snippet",
	nginxAnnotationPrefix + "stream-snippet",
}

type IngressRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewIngressRule(excludeRules []pkg.KindRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *IngressRule {
	return &IngressRule{
		RuleMeta: pkg.RuleMeta{
			Name: IngressRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(IngressRuleName),
	}
}

var _ pkg.Rule = (*IngressRule)(nil)

func (r *IngressRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		r.checkObject(object)
	}
}

func (r *IngressRule) checkObject(object storage.StoreObject) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	if object.Unstructured.GetKind() != "Ingress" {
		return
	}

	if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
		log.Info("⚠️ Skip Ingress due to exclusion rule", slog.String("name", object.Unstructured.GetName()))
		return
	}

	converter := runtime.DefaultUnstructuredConverter

	ingress := new(v1.Ingress)
	if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), ingress); err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	annotations := ingress.GetAnnotations()
	objectErrors := errorList.WithObjectID(object.Unstructured.GetName())

	for _, annotation := range unsafeIngressAnnotations {
		if _, found := annotations[annotation]; !found {
			continue
		}

		objectErrors.Errorf("Ingress annotation %q is unsafe and requires manual migration.", annotation)
	}

	configurationSnippet, found := annotations[configurationSnippetAnnotation]
	if !found {
		return
	}

	hasSafeHSTS := annotations[ingressNginxHSTSAnnotation] == "true"

	hasLegacyHSTS := strings.Contains(configurationSnippet, legacyHSTSDirective)
	if !hasSafeHSTS && !hasLegacyHSTS {
		objectErrors.Errorf("Ingress annotation %q requires annotation %q to be set to %q to preserve HSTS.",
			configurationSnippetAnnotation, ingressNginxHSTSAnnotation, "true")
	}
}
