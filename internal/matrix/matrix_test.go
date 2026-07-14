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

package matrix

import (
	"os"
	"path/filepath"
	"testing"
)

const configValues = `
type: object
properties:
  debug:
    type: boolean
  internal:
    type: object
    properties:
      activated:
        type: boolean
        x-examples: [false, true]
  resourcesRequests:
    type: object
    x-examples:
      - mode: VPA
        vpa:
          mode: Auto
      - mode: Static
        static:
          cpu: "55m"
          memory: "256Ki"
`

const valuesSchema = `
x-extend:
  schema: config-values.yaml
type: object
properties: {}
`

func writeTestModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	openapi := filepath.Join(dir, "openapi")

	if err := os.MkdirAll(openapi, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("module.yaml", "name: test-module\nnamespace: test-module\n")
	write(filepath.Join("openapi", "config-values.yaml"), configValues)
	write(filepath.Join("openapi", "values.yaml"), valuesSchema)

	return dir
}

func TestGenerate_IncludesDefaultAndCombos(t *testing.T) {
	dir := writeTestModule(t)

	variants, err := Generate(dir, "values.yaml", 100)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(variants) < 2 {
		t.Fatalf("expected default + combinations, got %d variants", len(variants))
	}

	if variants[0].Overrides != nil {
		t.Errorf("first variant must be the default (nil overrides), got %v", variants[0].Overrides)
	}

	// There must be a variant that simultaneously activates the module and
	// selects the Static resources example — the combination that reaches the
	// conditionally-rendered resource.
	found := false

	for _, v := range variants {
		mv, ok := v.Overrides["testModule"].(map[string]any)
		if !ok {
			continue
		}

		internal, _ := mv["internal"].(map[string]any)
		activated, _ := internal["activated"].(bool)

		rr, _ := mv["resourcesRequests"].(map[string]any)
		mode, _ := rr["mode"].(string)

		if activated && mode == "Static" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("no variant combined internal.activated=true with resourcesRequests.mode=Static; variants:\n%s", labels(variants))
	}
}

func labels(variants []Variant) string {
	out := ""
	for _, v := range variants {
		out += "  - " + v.Label + "\n"
	}

	return out
}

const oneOfConfigValues = `
type: object
properties:
  resourcesMode:
    type: object
    default: {}
    oneOf:
      - properties:
          mode:
            enum: ["Balanced"]
      - properties:
          mode:
            enum: ["Static"]
    properties:
      mode:
        type: string
        enum: ["Balanced", "Static"]
        default: "Balanced"
`

func TestGenerate_ExpandsOneOf(t *testing.T) {
	dir := t.TempDir()
	openapi := filepath.Join(dir, "openapi")

	if err := os.MkdirAll(openapi, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("module.yaml", "name: one-of\nnamespace: one-of\n")
	writeFile(filepath.Join("openapi", "config-values.yaml"), oneOfConfigValues)
	writeFile(filepath.Join("openapi", "values.yaml"), valuesSchema)

	variants, err := Generate(dir, "values.yaml", 100)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// The non-default (generated Balanced) "Static" branch must be reachable in
	// at least one variant, either via the oneOf branch value or the mode enum.
	found := false

	for _, v := range variants {
		mv, ok := v.Overrides["oneOf"].(map[string]any)
		if !ok {
			continue
		}

		rm, ok := mv["resourcesMode"].(map[string]any)
		if !ok {
			continue
		}

		if mode, _ := rm["mode"].(string); mode == "Static" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("no variant selected the Static oneOf/enum branch; variants:\n%s", labels(variants))
	}
}

func TestSetPath_Nested(t *testing.T) {
	root := map[string]any{}
	setPath(root, []string{"internal", "activated"}, true)
	setPath(root, []string{"internal", "other"}, "x")

	internal, ok := root["internal"].(map[string]any)
	if !ok {
		t.Fatal("internal not created as map")
	}

	if internal["activated"] != true || internal["other"] != "x" {
		t.Fatalf("nested values not set correctly: %v", internal)
	}
}
