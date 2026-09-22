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

package bootstrap

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

// Marshal renders the declaration as rbac.yaml, with the reader's homework as a comment on top.
func Marshal(r Result) ([]byte, error) {
	var head strings.Builder

	head.WriteString("# Written by dmt (rbac/sync --fix) from the RBAC objects the module rendered. Review it, resolve every TODO\n")
	head.WriteString("# and every note below, then run \"dmt lint --linter rbac --fix\" to regenerate the templates from it.\n")

	if len(r.Notes) > 0 {
		head.WriteString("#\n# Notes:\n")

		for _, n := range r.Notes {
			head.WriteString("# - " + n + "\n")
		}
	}

	if len(r.Unmanaged) > 0 {
		head.WriteString("#\n# Not described by the declaration (stays hand-written, as it is):\n")

		for _, u := range r.Unmanaged {
			head.WriteString("# - " + u + "\n")
		}
	}

	var body bytes.Buffer

	enc := yaml.NewEncoder(&body)
	enc.SetIndent(2)

	if err := enc.Encode(r.Decl); err != nil {
		return nil, err
	}

	if err := enc.Close(); err != nil {
		return nil, err
	}

	return []byte(head.String() + body.String()), nil
}
