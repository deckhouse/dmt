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
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func httpRouteObject(name string, parentRefs, rules []map[string]any) storage.StoreObject {
	u := unstructured.Unstructured{}
	u.SetKind(HTTPRouteKind)
	u.SetName(name)
	u.SetNamespace(testLSNamespace)

	if parentRefs != nil {
		_ = unstructured.SetNestedSlice(u.Object, toAnySlice(parentRefs), "spec", "parentRefs")
	}

	if rules != nil {
		_ = unstructured.SetNestedSlice(u.Object, toAnySlice(rules), "spec", "rules")
	}

	return storage.StoreObject{Unstructured: u, AbsPath: "/test/" + name + ".yaml"}
}

func listenerSetParentRef(sectionName string) map[string]any {
	return map[string]any{
		"group":       "gateway.networking.k8s.io",
		"kind":        ListenerSetKind,
		"name":        testLSName,
		"namespace":   testLSNamespace,
		"sectionName": sectionName,
	}
}

func redirectRule(scheme string, port int64) map[string]any {
	requestRedirect := map[string]any{"statusCode": int64(301)}
	if scheme != "" {
		requestRedirect["scheme"] = scheme
	}

	if port != 0 {
		requestRedirect["port"] = port
	}

	return map[string]any{
		"filters": []any{
			map[string]any{"type": requestRedirectFilterType, "requestRedirect": requestRedirect},
		},
	}
}

func backendRule() map[string]any {
	return map[string]any{
		"backendRefs": []any{
			map[string]any{"kind": "Service", "name": "kiali", "port": int64(443)},
		},
	}
}

// redirectListenerSet is a ListenerSet with one HTTPS host and its port 80 redirect
// section (istio-redirect), the shape both redirect rules expect.
func redirectListenerSet() storage.StoreObject {
	return listenerSetObject([]map[string]any{
		httpsListener("istio", "grafana.example.com"),
		httpRedirectListener("istio-redirect", "grafana.example.com"),
	})
}

func runHTTPRouteRedirectRule(t *testing.T, exclude []pkg.HTTPRouteRedirectExclude, objects ...storage.StoreObject) *errors.LintRuleErrorsList {
	t.Helper()

	mc := minimock.NewController(t)
	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storeFrom(objects...))

	errorList := errors.NewLintRuleErrorsList()
	NewHTTPRouteRedirectRule(exclude, mod, errorList).Check(t.Context())

	return errorList
}

func TestHTTPRouteRedirectRule_ValidRedirectRoute(t *testing.T) {
	route := httpRouteObject("kiali-redirect",
		[]map[string]any{listenerSetParentRef("istio-redirect")},
		[]map[string]any{redirectRule(httpsScheme, 0)},
	)

	assert.False(t, runHTTPRouteRedirectRule(t, nil, redirectListenerSet(), route).ContainsErrors())
}

func TestHTTPRouteRedirectRule_MissingRedirectRoute(t *testing.T) {
	assert.True(t, runHTTPRouteRedirectRule(t, nil, redirectListenerSet()).ContainsErrors())
}

func TestHTTPRouteRedirectRule_RouteTargetsWrongSection(t *testing.T) {
	route := httpRouteObject("kiali-redirect",
		[]map[string]any{listenerSetParentRef("some-other-section")},
		[]map[string]any{redirectRule(httpsScheme, 0)},
	)

	assert.True(t, runHTTPRouteRedirectRule(t, nil, redirectListenerSet(), route).ContainsErrors())
}

func TestHTTPRouteRedirectRule_RouteWithoutRedirectFilter(t *testing.T) {
	// Targets the redirect section but serves traffic instead of redirecting.
	route := httpRouteObject("kiali",
		[]map[string]any{listenerSetParentRef("istio-redirect")},
		[]map[string]any{backendRule()},
	)

	assert.True(t, runHTTPRouteRedirectRule(t, nil, redirectListenerSet(), route).ContainsErrors())
}

func TestHTTPRouteRedirectRule_RedirectByPort443(t *testing.T) {
	route := httpRouteObject("kiali-redirect",
		[]map[string]any{listenerSetParentRef("istio-redirect")},
		[]map[string]any{redirectRule("", 443)},
	)

	assert.False(t, runHTTPRouteRedirectRule(t, nil, redirectListenerSet(), route).ContainsErrors())
}

func TestHTTPRouteRedirectRule_HTTPOnlyHostNeedsNoRedirect(t *testing.T) {
	// A plain HTTP host (no HTTPS listener) uses its port 80 listener to serve, not
	// to redirect, so no redirecting HTTPRoute is required.
	ls := listenerSetObject([]map[string]any{
		httpRedirectListener("istio", "plain.example.com"),
	})

	assert.False(t, runHTTPRouteRedirectRule(t, nil, ls).ContainsErrors())
}

func TestHTTPRouteRedirectRule_ExcludedWholeListenerSet(t *testing.T) {
	// Section wildcard silences every redirect section of the ListenerSet.
	exclude := []pkg.HTTPRouteRedirectExclude{{ListenerSet: "istio"}}

	assert.False(t, runHTTPRouteRedirectRule(t, exclude, redirectListenerSet()).ContainsErrors())
}

func TestHTTPRouteRedirectRule_ExcludedSingleSection(t *testing.T) {
	exclude := []pkg.HTTPRouteRedirectExclude{{ListenerSet: "istio", Section: "istio-redirect"}}

	assert.False(t, runHTTPRouteRedirectRule(t, exclude, redirectListenerSet()).ContainsErrors())
}

func TestHTTPRouteRedirectRule_ExcludedSectionKeepsOthersChecked(t *testing.T) {
	// Two redirect sections; excluding one must leave the other flagged.
	ls := listenerSetObject([]map[string]any{
		httpsListener("istio", "grafana.example.com"),
		httpRedirectListener("istio-redirect", "grafana.example.com"),
		httpsListener("api-proxy", "api.example.com"),
		httpRedirectListener("api-proxy-redirect", "api.example.com"),
	})

	exclude := []pkg.HTTPRouteRedirectExclude{{Section: "istio-redirect"}}
	errorList := runHTTPRouteRedirectRule(t, exclude, ls)
	found := errorList.GetErrors()
	assert.Len(t, found, 1)
	assert.Contains(t, found[0].Text, "api-proxy-redirect")
}
