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

func toAnySlice(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}

	return out
}

// Test ListenerSet identity. Fixed literals because these rules key on section
// names, not on the ListenerSet name/namespace, so varying them adds nothing.
const (
	testLSName      = "istio"
	testLSNamespace = "d8-istio"
)

func listenerSetObject(listeners []map[string]any) storage.StoreObject {
	u := unstructured.Unstructured{}
	u.SetKind(ListenerSetKind)
	u.SetName(testLSName)
	u.SetNamespace(testLSNamespace)

	_ = unstructured.SetNestedSlice(u.Object, toAnySlice(listeners), "spec", "listeners")

	return storage.StoreObject{Unstructured: u, AbsPath: "/test/" + testLSName + ".yaml"}
}

func storeFrom(objects ...storage.StoreObject) map[storage.ResourceIndex]storage.StoreObject {
	store := make(map[storage.ResourceIndex]storage.StoreObject, len(objects))
	for _, obj := range objects {
		store[storage.ResourceIndex{Kind: obj.Unstructured.GetKind(), Name: obj.Unstructured.GetName()}] = obj
	}

	return store
}

func httpsListenerPort(name, hostname string, port int64) map[string]any {
	return map[string]any{"name": name, "hostname": hostname, "port": port, "protocol": httpsProtocol}
}

func httpsListener(name, hostname string) map[string]any {
	return httpsListenerPort(name, hostname, httpsPort)
}

func httpRedirectListener(name, hostname string) map[string]any {
	return map[string]any{"name": name, "hostname": hostname, "port": int64(80), "protocol": httpProtocol}
}

func runListenerSetRedirectRule(t *testing.T, exclude []pkg.ListenerSetRedirectExclude, objects ...storage.StoreObject) *errors.LintRuleErrorsList {
	t.Helper()

	mc := minimock.NewController(t)
	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storeFrom(objects...))

	errorList := errors.NewLintRuleErrorsList()
	NewListenerSetRedirectRule(exclude, mod, errorList).Check(t.Context())

	return errorList
}

func TestListenerSetRedirectRule_HTTPSWithMatchingHTTP(t *testing.T) {
	ls := listenerSetObject([]map[string]any{
		httpsListener("istio", "grafana.example.com"),
		httpRedirectListener("istio-redirect", "grafana.example.com"),
	})

	assert.False(t, runListenerSetRedirectRule(t, nil, ls).ContainsErrors())
}

func TestListenerSetRedirectRule_HTTPSWithoutHTTP(t *testing.T) {
	ls := listenerSetObject([]map[string]any{
		httpsListener("istio-metadata", "metadata.example.com"),
	})

	assert.True(t, runListenerSetRedirectRule(t, nil, ls).ContainsErrors())
}

func TestListenerSetRedirectRule_HTTPOnlyHostNeedsNoRedirect(t *testing.T) {
	ls := listenerSetObject([]map[string]any{
		httpRedirectListener("istio", "plain.example.com"),
	})

	assert.False(t, runListenerSetRedirectRule(t, nil, ls).ContainsErrors())
}

func TestListenerSetRedirectRule_ReportsEachHostOnce(t *testing.T) {
	// Gateway API requires HTTPS listeners to be distinct by (protocol, port, hostname),
	// so a single host can only be served by more than one HTTPS listener on DISTINCT
	// ports. Two such HTTPS listeners for one host with no port-80 HTTP counterpart still
	// yield a single finding (deduped by hostname).
	ls := listenerSetObject([]map[string]any{
		httpsListenerPort("secure", "same.example.com", httpsPort),
		httpsListenerPort("secure-alt", "same.example.com", int64(8443)),
	})

	errorList := runListenerSetRedirectRule(t, nil, ls)
	assert.Len(t, errorList.GetErrors(), 1)
}

func TestListenerSetRedirectRule_ExcludedWholeListenerSet(t *testing.T) {
	ls := listenerSetObject([]map[string]any{
		httpsListener("istio-metadata", "metadata.example.com"),
	})

	// An exclude with only the ListenerSet name (Section wildcard) silences the whole set.
	exclude := []pkg.ListenerSetRedirectExclude{{ListenerSet: "istio"}}
	assert.False(t, runListenerSetRedirectRule(t, exclude, ls).ContainsErrors())
}

func TestListenerSetRedirectRule_ExcludedSingleSection(t *testing.T) {
	ls := listenerSetObject([]map[string]any{
		httpsListener("istio-metadata", "metadata.example.com"),
	})

	// Keyed by section name, not hostname (hostnames are unknown before rendering).
	exclude := []pkg.ListenerSetRedirectExclude{{ListenerSet: "istio", Section: "istio-metadata"}}
	assert.False(t, runListenerSetRedirectRule(t, exclude, ls).ContainsErrors())
}

func TestListenerSetRedirectRule_ExcludedSectionKeepsOthersChecked(t *testing.T) {
	// Two HTTPS-only sections in one ListenerSet. Excluding only "istio-metadata"
	// must leave "other" still flagged - a whole-ListenerSet exclude would wrongly
	// silence both.
	ls := listenerSetObject([]map[string]any{
		httpsListener("istio-metadata", "metadata.example.com"),
		httpsListener("other", "other.example.com"),
	})

	exclude := []pkg.ListenerSetRedirectExclude{{Section: "istio-metadata"}}
	errorList := runListenerSetRedirectRule(t, exclude, ls)
	found := errorList.GetErrors()
	assert.Len(t, found, 1)
	assert.Contains(t, found[0].Text, "other.example.com")
}
