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

package config

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
)

// TestRedirectExcludeRulesDecodeFromConfigMap decodes the exclude-rules block a
// user writes in .dmtlint.yaml the same way viper does, asserting that both the
// section-list keys (listenerset-redirect / httproute-redirect) and the per-entry
// `name`/`section` field tags land on the right struct fields. A wrong mapstructure
// tag would compile and pass every rule unit test (which build pkg.*Exclude
// directly), so this is the only place that catches it.
func TestRedirectExcludeRulesDecodeFromConfigMap(t *testing.T) {
	raw := map[string]any{
		"listenerset-redirect": []map[string]any{
			{"name": "istio", "section": "istio-metadata"},
			{"section": "any-set-wildcard"}, // empty name -> any ListenerSet
		},
		"httproute-redirect": []map[string]any{
			{"name": "istio", "section": "istio-redirect"},
		},
	}

	var excludeRules TemplatesExcludeRules
	require.NoError(t, mapstructure.Decode(raw, &excludeRules))

	require.Equal(t, ListenerSetRedirectExcludeList{
		{ListenerSet: "istio", Section: "istio-metadata"},
		{ListenerSet: "", Section: "any-set-wildcard"},
	}, excludeRules.ListenerSetRedirect)

	require.Equal(t, HTTPRouteRedirectExcludeList{
		{ListenerSet: "istio", Section: "istio-redirect"},
	}, excludeRules.HTTPRouteRedirect)
}
