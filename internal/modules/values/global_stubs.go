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

package values

import (
	_ "embed"
	"fmt"

	"sigs.k8s.io/yaml"
)

//go:embed global_stubs.yaml
var globalStubsYAML []byte

// globalStubs returns a fresh copy of the embedded .Values.global.* placeholder
// values. dmt renders strictly, so every global key deckhouse_lib_helm indexes
// must exist and be well-typed; these defaults fill the gaps a module's own
// generated values don't cover. Parsed per call so callers may mutate the result.
func globalStubs() (map[string]any, error) {
	stubs := map[string]any{}
	if err := yaml.Unmarshal(globalStubsYAML, &stubs); err != nil {
		return nil, fmt.Errorf("parse global stubs: %w", err)
	}

	return stubs, nil
}
