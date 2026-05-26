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

package rules

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	HTTPRouteRuleName = "httproute-rules"
	IngressKind       = "Ingress"
	HTTPRouteKind     = "HTTPRoute"
	ListenerSetKind   = "ListenerSet"
	AppLabelKey       = "app"
)

type HTTPRouteRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

func NewHTTPRouteRule(excludeRules []pkg.KindRuleExclude) *HTTPRouteRule {
	return &HTTPRouteRule{
		RuleMeta: pkg.RuleMeta{
			Name: HTTPRouteRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

func (r *HTTPRouteRule) ModuleMustHaveGatewayResources(md pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	httpRoutes := collectStoreObjectsByKind(md, HTTPRouteKind)
	listenerSets := collectStoreObjectsByKind(md, ListenerSetKind)

	for _, object := range md.GetStorage() {
		if object.Unstructured.GetKind() != IngressKind {
			continue
		}

		name := object.Unstructured.GetName()
		if !r.Enabled(IngressKind, name) {
			continue
		}
		errorListObj := errorList.WithObjectID(object.Identity()).WithFilePath(object.GetPath())

		route, ok := findHTTPRouteByLabels(object, httpRoutes)
		if !ok {
			errorListObj.Errorf("Ingress %q requires a matching HTTPRoute with the same app label, but none was found", name)
			continue
		}

		if err := validateHTTPRouteParentRefs(route, listenerSets); err != nil {
			errorList.WithObjectID(route.Identity()).
				WithFilePath(route.GetPath()).
				Errorf("HTTPRoute %q is invalid for Ingress migration: %v", route.Unstructured.GetName(), err)
		}
	}
}

func collectStoreObjectsByKind(md pkg.Module, kind string) []storage.StoreObject {
	var objects []storage.StoreObject

	for _, object := range md.GetStorage() {
		if object.Unstructured.GetKind() == kind {
			objects = append(objects, object)
		}
	}

	return objects
}

func findHTTPRouteByLabels(ingress storage.StoreObject, routes []storage.StoreObject) (storage.StoreObject, bool) {
	ingressAppLabel := ingress.Unstructured.GetLabels()[AppLabelKey]
	if ingressAppLabel == "" {
		return storage.StoreObject{}, false
	}

	for _, route := range routes {
		if route.Unstructured.GetLabels()[AppLabelKey] == ingressAppLabel {
			return route, true
		}
	}

	return storage.StoreObject{}, false
}

func validateHTTPRouteParentRefs(
	route storage.StoreObject,
	listenerSets []storage.StoreObject,
) error {
	parentRefs, found, err := unstructured.NestedSlice(route.Unstructured.Object, "spec", "parentRefs")
	if err != nil {
		return fmt.Errorf("cannot read spec.parentRefs: %w", err)
	}

	if !found || len(parentRefs) == 0 {
		return fmt.Errorf("spec.parentRefs must reference an existing ListenerSet")
	}

	for _, parent := range parentRefs {
		parentMap, ok := parent.(map[string]any)
		if !ok {
			continue
		}

		name, ok := parentMap["name"].(string)
		if !ok || name == "" {
			continue
		}

		for _, listenerSet := range listenerSets {
			if listenerSet.Unstructured.GetName() == name {
				return nil
			}
		}
	}

	return fmt.Errorf("spec.parentRefs does not reference any ListenerSet found in module templates")
}
