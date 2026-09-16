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
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	HTTPRouteRedirectRuleName = "httproute-redirect"

	requestRedirectFilterType = "RequestRedirect"
	httpsScheme               = "https"
)

// HTTPRouteRedirectRule enforces that every plain-HTTP (port 80) redirect section
// of a ListenerSet — one whose hostname is also served over HTTPS — is backed by
// an HTTPRoute that redirects it to HTTPS. It complements ListenerSetRedirectRule:
// that rule guarantees the port 80 listener exists, this one guarantees something
// actually redirects it (an HTTPRoute whose parentRef targets the section with a
// RequestRedirect filter to https/443).
type HTTPRouteRedirectRule struct {
	pkg.RuleMeta

	excludeRules []pkg.HTTPRouteRedirectExclude
	module       pkg.Module
	errorList    *errors.LintRuleErrorsList
}

func NewHTTPRouteRedirectRule(excludeRules []pkg.HTTPRouteRedirectExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *HTTPRouteRedirectRule {
	return &HTTPRouteRedirectRule{
		RuleMeta: pkg.RuleMeta{
			Name: HTTPRouteRedirectRuleName,
		},
		excludeRules: excludeRules,
		module:       m,
		errorList:    errorList.WithRule(HTTPRouteRedirectRuleName),
	}
}

var _ pkg.Rule = (*HTTPRouteRedirectRule)(nil)

// enabled reports whether the (ListenerSet, section) pair is still checked, i.e.
// not silenced by any exclusion rule.
func (r *HTTPRouteRedirectRule) enabled(listenerSet, section string) bool {
	for i := range r.excludeRules {
		if !r.excludeRules[i].Enabled(listenerSet, section) {
			return false
		}
	}

	return true
}

func (r *HTTPRouteRedirectRule) Check(_ context.Context) {
	listenerSets := collectStoreObjectsByKind(r.module, ListenerSetKind)
	httpRoutes := collectStoreObjectsByKind(r.module, HTTPRouteKind)

	for _, ls := range listenerSets {
		lsName := ls.Unstructured.GetName()
		lsNamespace := ls.Unstructured.GetNamespace()
		listeners := listenerSetListeners(ls)
		httpsHosts := httpsHostnames(listeners)

		reported := make(map[string]bool)

		for _, l := range listeners {
			// Only an HTTP (port 80) listener whose host is also served over HTTPS is
			// a redirect section. A host served over plain HTTP only uses its port 80
			// listener to serve real traffic, not to redirect, so it needs no
			// redirecting HTTPRoute.
			if !l.isHTTPRedirect() || !httpsHosts[l.hostname] {
				continue
			}

			if l.name == "" || reported[l.name] {
				continue
			}

			if !r.enabled(lsName, l.name) {
				continue
			}

			if hasHTTPSRedirectRoute(httpRoutes, lsName, lsNamespace, l.name) {
				continue
			}

			reported[l.name] = true
			r.errorList.WithObjectID(ls.Identity()).
				WithFilePath(ls.GetPath()).
				WithValue(l.name).
				Errorf(
					"ListenerSet %q section %q is an HTTP (port 80) redirect listener for host %q, but no HTTPRoute redirects it to HTTPS; "+
						"add an HTTPRoute whose parentRef targets this ListenerSet section (name: %q, sectionName: %q) with a "+
						"RequestRedirect filter to HTTPS (scheme: https).",
					lsName, l.name, l.hostname, lsName, l.name,
				)
		}
	}
}

// hasHTTPSRedirectRoute reports whether any HTTPRoute targets the given ListenerSet
// section via parentRef and carries a RequestRedirect-to-HTTPS filter.
func hasHTTPSRedirectRoute(routes []storage.StoreObject, lsName, lsNamespace, sectionName string) bool {
	for _, route := range routes {
		if httpRouteTargetsSection(route, lsName, lsNamespace, sectionName) && httpRouteHasHTTPSRedirect(route) {
			return true
		}
	}

	return false
}

// httpRouteTargetsSection reports whether the route has a parentRef pointing at
// the named ListenerSet section. A parentRef with an empty kind is treated as a
// ListenerSet reference (Gateway API defaults are not resolved here); a
// parentRef with an omitted namespace defaults to the route's own namespace.
func httpRouteTargetsSection(route storage.StoreObject, lsName, lsNamespace, sectionName string) bool {
	parentRefs, found, err := unstructured.NestedSlice(route.Unstructured.Object, "spec", "parentRefs")
	if err != nil || !found {
		return false
	}

	for _, parent := range parentRefs {
		parentMap, ok := parent.(map[string]any)
		if !ok {
			continue
		}

		if kind := mapString(parentMap, "kind"); kind != "" && kind != ListenerSetKind {
			continue
		}

		if mapString(parentMap, "name") != lsName {
			continue
		}

		if mapString(parentMap, "sectionName") != sectionName {
			continue
		}

		namespace := mapString(parentMap, "namespace")
		if namespace == "" {
			namespace = route.Unstructured.GetNamespace()
		}

		if lsNamespace != "" && namespace != "" && namespace != lsNamespace {
			continue
		}

		return true
	}

	return false
}

// httpRouteHasHTTPSRedirect reports whether the route has a rule with a
// RequestRedirect filter that redirects to HTTPS (scheme https, or port 443).
func httpRouteHasHTTPSRedirect(route storage.StoreObject) bool {
	rules, found, err := unstructured.NestedSlice(route.Unstructured.Object, "spec", "rules")
	if err != nil || !found {
		return false
	}

	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}

		filters, ok := ruleMap["filters"].([]any)
		if !ok {
			continue
		}

		for _, filter := range filters {
			filterMap, ok := filter.(map[string]any)
			if !ok {
				continue
			}

			if mapString(filterMap, "type") != requestRedirectFilterType {
				continue
			}

			redirect, ok := filterMap["requestRedirect"].(map[string]any)
			if !ok {
				continue
			}

			if strings.EqualFold(mapString(redirect, "scheme"), httpsScheme) {
				return true
			}

			if port, ok := mapInt64(redirect["port"]); ok && port == httpsPort {
				return true
			}
		}
	}

	return false
}
