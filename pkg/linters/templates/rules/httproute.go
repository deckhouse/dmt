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

	// networkTeamHint is appended to every finding so module authors know who
	// can help with the Ingress -> Gateway API migration and review the change.
	networkTeamHint = "If you need help with the Gateway API migration or a review, reach out to the network team in #dev-network."
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
			errorListObj.Errorf(
				"Ingress %q ships in this module, but the corresponding Gateway API resources are missing: "+
					"no HTTPRoute with a matching %q=%q label was found. "+
					"Every Ingress must be accompanied by a Gateway API HTTPRoute (referencing a ListenerSet) so the module is ready for the Ingress -> Gateway API migration. %s",
				name, AppLabelKey, object.Unstructured.GetLabels()[AppLabelKey], networkTeamHint,
			)
			continue
		}

		if err := validateHTTPRouteParentRefs(route, listenerSets); err != nil {
			errorList.WithObjectID(route.Identity()).
				WithFilePath(route.GetPath()).
				Errorf(
					"HTTPRoute %q does not yet provide a valid Gateway API replacement for Ingress %q: %v. %s",
					route.Unstructured.GetName(), name, err, networkTeamHint,
				)
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
		return fmt.Errorf("spec.parentRefs is empty, so it does not reference a ListenerSet defined in the module templates")
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

	return fmt.Errorf("none of the spec.parentRefs reference a ListenerSet defined in the module templates")
}
