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
	"strings"

	"github.com/Masterminds/sprig/v3"
)

// sprigFuncs backs ModuleCamelName. camelcase and untitle come from sprig — the
// same helpers deckhouse_lib_helm uses — so ModuleCamelName matches helm_lib.
var sprigFuncs = sprig.TxtFuncMap()

// ModuleCamelName converts a module name to the camelCase key deckhouse_lib_helm
// uses for it. It mirrors helm_lib_module_camelcase_name exactly
// (name | replace "-" "_" | camelcase | untitle), which matters because dmt keys
// the module's render values — image digests and per-module settings — under this
// name so the real helm_lib helpers resolve them at render time. ToLowerCamel here
// is unsuitable: it inserts word boundaries around digits (l2 -> l2, but e2e -> e2E),
// diverging from helm_lib on names like "l2-load-balancer".
func ModuleCamelName(name string) string {
	camelcase := sprigFuncs["camelcase"].(func(string) string)
	untitle := sprigFuncs["untitle"].(func(string) string)

	return untitle(camelcase(strings.ReplaceAll(name, "-", "_")))
}
