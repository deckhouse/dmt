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
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ListenerSetRedirectRuleName = "listenerset-redirect"

	httpProtocol  = "HTTP"
	httpsProtocol = "HTTPS"
	httpPort      = int64(80)
	httpsPort     = int64(443)
)

// listenerSetListener is one entry of a ListenerSet's spec.listeners, reduced to
// the fields the redirect rules care about.
type listenerSetListener struct {
	name     string
	hostname string
	protocol string
	port     int64
}

// isHTTPRedirect reports whether this listener is the plain-HTTP (port 80)
// listener a host uses to redirect to HTTPS.
func (l listenerSetListener) isHTTPRedirect() bool {
	return strings.EqualFold(l.protocol, httpProtocol) && l.port == httpPort
}

func (l listenerSetListener) isHTTPS() bool {
	return strings.EqualFold(l.protocol, httpsProtocol)
}

// listenerSetListeners extracts spec.listeners from a ListenerSet object.
func listenerSetListeners(obj storage.StoreObject) []listenerSetListener {
	raw, found, err := unstructured.NestedSlice(obj.Unstructured.Object, "spec", "listeners")
	if err != nil || !found {
		return nil
	}

	listeners := make([]listenerSetListener, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		l := listenerSetListener{
			name:     mapString(m, "name"),
			hostname: mapString(m, "hostname"),
			protocol: mapString(m, "protocol"),
		}
		if port, ok := mapInt64(m["port"]); ok {
			l.port = port
		}

		listeners = append(listeners, l)
	}

	return listeners
}

// httpsHostnames returns the set of hostnames served by an HTTPS listener.
func httpsHostnames(listeners []listenerSetListener) map[string]bool {
	hosts := make(map[string]bool)

	for _, l := range listeners {
		if l.isHTTPS() {
			hosts[l.hostname] = true
		}
	}

	return hosts
}

func mapString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// mapInt64 normalizes the several numeric shapes a YAML/JSON-decoded value can
// take (int64/int/float64/json.Number) into an int64.
func mapInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

// ListenerSetRedirectRule enforces that every hostname a ListenerSet exposes over
// HTTPS also has a plain-HTTP (port 80) listener, so that plain HTTP requests to
// that host can be redirected to HTTPS instead of being refused. The port 80
// listener itself is wired to a redirect by the httproute-redirect rule.
type ListenerSetRedirectRule struct {
	pkg.RuleMeta

	excludeRules []pkg.ListenerSetRedirectExclude
	module       pkg.Module
	errorList    *errors.LintRuleErrorsList
}

func NewListenerSetRedirectRule(excludeRules []pkg.ListenerSetRedirectExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *ListenerSetRedirectRule {
	return &ListenerSetRedirectRule{
		RuleMeta: pkg.RuleMeta{
			Name: ListenerSetRedirectRuleName,
		},
		excludeRules: excludeRules,
		module:       m,
		errorList:    errorList.WithRule(ListenerSetRedirectRuleName),
	}
}

var _ pkg.Rule = (*ListenerSetRedirectRule)(nil)

// enabled reports whether the (ListenerSet, section) pair is still checked, i.e.
// not silenced by any exclusion rule. It is keyed by the HTTPS listener's section
// name, since hostnames are not known before rendering.
func (r *ListenerSetRedirectRule) enabled(listenerSet, section string) bool {
	for i := range r.excludeRules {
		if !r.excludeRules[i].Enabled(listenerSet, section) {
			return false
		}
	}

	return true
}

func (r *ListenerSetRedirectRule) Check(_ context.Context) {
	for _, obj := range collectStoreObjectsByKind(r.module, ListenerSetKind) {
		name := obj.Unstructured.GetName()

		listeners := listenerSetListeners(obj)

		httpHosts := make(map[string]bool)

		for _, l := range listeners {
			if l.isHTTPRedirect() {
				httpHosts[l.hostname] = true
			}
		}

		// One finding per hostname missing its HTTP counterpart, even if the host
		// declares several HTTPS listeners. Exclusions are keyed by the HTTPS
		// listener's section name (the stable, render-independent identifier).
		reported := make(map[string]bool)
		for _, l := range listeners {
			if !l.isHTTPS() || httpHosts[l.hostname] || reported[l.hostname] {
				continue
			}

			if !r.enabled(name, l.name) {
				continue
			}

			reported[l.hostname] = true
			r.errorList.WithObjectID(obj.Identity()).
				WithFilePath(obj.GetPath()).
				WithValue(l.name).
				Errorf(
					"ListenerSet %q exposes host %q over HTTPS via listener section %q but has no HTTP (port 80) listener for the same host; "+
						"add a port 80 HTTP listener section so plain HTTP requests to %q can be redirected to HTTPS.",
					name, l.hostname, l.name, l.hostname,
				)
		}
	}
}
