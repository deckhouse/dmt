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

package schemas

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"sigs.k8s.io/yaml"
)

// LoadModuleCRDs parses every CustomResourceDefinition found under dir (the
// module's crds/ directory) and registers a compiled schema for each of its
// served versions. These take precedence over the bundled catalog. A missing
// directory is not an error. Individual CRDs that fail to parse or compile are
// skipped and reported through the returned (aggregated) error, so a single bad
// file never prevents the rest from being used.
func (s *Store) LoadModuleCRDs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read crds dir: %w", err)
	}

	var problems []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		// Deckhouse ships doc-only helper files (e.g. doc-ru-*.yaml) alongside
		// CRDs; they are not definitions and are ignored below by kind.
		path := filepath.Join(dir, name)

		content, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %s", name, err))
			continue
		}

		for _, doc := range splitYAML(string(content)) {
			if err := s.loadCRDDoc([]byte(doc)); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %s", name, err))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("load module CRDs: %s", strings.Join(problems, "; "))
	}

	return nil
}

// loadCRDDoc registers the schemas of a single CRD document as the module's own
// (authoritative) CRDs, recording any null-keyword notes for the caller.
func (s *Store) loadCRDDoc(doc []byte) error {
	compiled, notes, err := compileCRDDoc(doc)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	maps.Copy(s.module, compiled)
	s.notes = append(s.notes, notes...)

	return nil
}

// compileCRDDoc parses a single CustomResourceDefinition document and compiles
// the OpenAPI v3 schema of each served version into a validator, keyed by lookup
// key. It also returns any null-keyword notes (see SchemaNote) gathered while
// sanitizing. Non-CRD documents yield an empty result and no error.
func compileCRDDoc(doc []byte) (map[string]*jsonschema.Schema, []SchemaNote, error) {
	var obj map[string]any
	if err := yaml.Unmarshal(doc, &obj); err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}

	if len(obj) == 0 {
		return nil, nil, nil
	}

	if kind, _ := obj["kind"].(string); kind != "CustomResourceDefinition" {
		return nil, nil, nil
	}

	spec, _ := obj["spec"].(map[string]any)
	if spec == nil {
		return nil, nil, nil
	}

	group, _ := spec["group"].(string)

	names, _ := spec["names"].(map[string]any)
	crdKind, _ := names["kind"].(string)

	if group == "" || crdKind == "" {
		return nil, nil, nil
	}

	schemasByVersion := crdVersionSchemas(spec)
	if len(schemasByVersion) == 0 {
		return nil, nil, nil
	}

	out := make(map[string]*jsonschema.Schema, len(schemasByVersion))

	var notes []SchemaNote

	for version, rawSchema := range schemasByVersion {
		sanitized, dropped := sanitizeOpenAPISchema(rawSchema)

		sch, err := compileFromDocument(sanitized)
		if err != nil {
			return nil, nil, fmt.Errorf("compile %s/%s %s: %w", group, version, crdKind, err)
		}

		out[crdLookupKey(group, version, crdKind)] = sch

		for _, path := range dropped {
			notes = append(notes, SchemaNote{
				Group:   group,
				Version: version,
				Kind:    crdKind,
				Path:    path,
			})
		}
	}

	return out, notes, nil
}

// crdVersionSchemas extracts the OpenAPI v3 schema for each served version of a
// CRD, supporting both the apiextensions.k8s.io/v1 layout (spec.versions[]) and
// the legacy v1beta1 layout (spec.validation with spec.version/spec.versions).
func crdVersionSchemas(spec map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any)

	if versions, ok := spec["versions"].([]any); ok {
		for _, v := range versions {
			vm, ok := v.(map[string]any)
			if !ok {
				continue
			}

			name, _ := vm["name"].(string)
			if name == "" {
				continue
			}

			schema, _ := vm["schema"].(map[string]any)

			openAPIV3Schema, _ := schema["openAPIV3Schema"].(map[string]any)
			if openAPIV3Schema != nil {
				out[name] = openAPIV3Schema
				continue
			}

			// A version without its own schema falls back to the legacy
			// top-level validation schema, mirroring apiserver behaviour.
			if legacy := legacyValidationSchema(spec); legacy != nil {
				out[name] = legacy
			}
		}
	}

	if len(out) == 0 {
		if legacy := legacyValidationSchema(spec); legacy != nil {
			if version, _ := spec["version"].(string); version != "" {
				out[version] = legacy
			}
		}
	}

	return out
}

func legacyValidationSchema(spec map[string]any) map[string]any {
	validation, _ := spec["validation"].(map[string]any)

	schema, _ := validation["openAPIV3Schema"].(map[string]any)

	return schema
}

// compileFromDocument compiles an already-decoded (map) JSON schema by
// re-encoding it to JSON and handing it to the shared compiler.
func compileFromDocument(doc map[string]any) (*jsonschema.Schema, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode schema: %w", err)
	}

	return compileSchema("crd://module", raw)
}

// sanitizeOpenAPISchema converts a Kubernetes OpenAPI v3 (structural) schema
// into a plain JSON Schema the validator can consume. It rewrites the
// Kubernetes-specific extensions that would otherwise cause false positives:
//
//   - x-kubernetes-preserve-unknown-fields: true relaxes additionalProperties;
//   - x-kubernetes-int-or-string: true drops the type constraint so both
//     integers and strings are accepted;
//   - nullable: true widens the declared type(s) to also permit null;
//   - all remaining x-kubernetes-* and non-validation annotation keys are
//     dropped;
//   - boolean exclusiveMinimum/Maximum (OpenAPI/draft-4 form) are dropped as
//     they are incompatible with modern JSON Schema drafts.
//
// The returned value is a deep copy; the input is not modified. dropped
// collects the paths of null-valued keywords that were removed (see sanitizeMap).
func sanitizeOpenAPISchema(in map[string]any) (out map[string]any, dropped []string) {
	sanitized, _ := sanitizeNode(in, "", &dropped).(map[string]any)
	if sanitized == nil {
		sanitized = map[string]any{}
	}

	return sanitized, dropped
}

func sanitizeNode(node any, path string, dropped *[]string) any {
	switch v := node.(type) {
	case map[string]any:
		return sanitizeMap(v, path, dropped)
	case []any:
		items := make([]any, 0, len(v))
		for i, item := range v {
			items = append(items, sanitizeNode(item, fmt.Sprintf("%s/%d", path, i), dropped))
		}

		return items
	default:
		return v
	}
}

func sanitizeMap(in map[string]any, path string, dropped *[]string) map[string]any {
	out := make(map[string]any, len(in))

	preserveUnknown := boolValue(in["x-kubernetes-preserve-unknown-fields"])
	intOrString := boolValue(in["x-kubernetes-int-or-string"])
	nullable := boolValue(in["nullable"])

	for key, value := range in {
		child := strings.TrimPrefix(path+"/"+key, "/")

		switch {
		case value == nil && key != "default" && key != "const":
			// A null keyword value (e.g. `maxLength:` or `description:` left empty
			// in a CRD) is invalid against the JSON Schema metaschema — maxLength
			// wants an integer, description a string — and carries no constraint
			// anyway. Drop it so the schema compiles instead of failing the whole
			// module's CRD load, and record the path so the caller can nudge the
			// author to clean it up. (default/const legitimately accept null; null
			// enum entries live inside arrays and are handled by sanitizeNode.)
			*dropped = append(*dropped, child)

			continue
		case strings.HasPrefix(key, "x-kubernetes-"):
			continue
		case key == "nullable":
			continue
		case key == "example", key == "default", key == "x-doc-example":
			// keep default (harmless), drop example-only annotations
			if key == "default" {
				out[key] = value
			}

			continue
		case (key == "exclusiveMinimum" || key == "exclusiveMaximum"):
			if _, isBool := value.(bool); isBool {
				continue
			}

			out[key] = sanitizeNode(value, child, dropped)
		default:
			out[key] = sanitizeNode(value, child, dropped)
		}
	}

	if intOrString {
		delete(out, "type")
		delete(out, "format")
	}

	if nullable {
		widenTypeWithNull(out)
	}

	if preserveUnknown {
		// Allow arbitrary extra properties instead of whatever constraint the
		// structural schema implied.
		out["additionalProperties"] = true
	}

	return out
}

// widenTypeWithNull augments a schema's "type" so that null becomes valid,
// matching the semantics of OpenAPI's nullable: true.
func widenTypeWithNull(schema map[string]any) {
	switch t := schema["type"].(type) {
	case string:
		if t != "null" {
			schema["type"] = []any{t, "null"}
		}
	case []any:
		for _, existing := range t {
			if s, ok := existing.(string); ok && s == "null" {
				return
			}
		}

		schema["type"] = append(t, "null")
	}
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}

// splitYAML splits a multi-document YAML string into its individual documents.
func splitYAML(content string) []string {
	var docs []string

	for part := range strings.SplitSeq("\n"+content, "\n---") {
		if strings.TrimSpace(part) != "" {
			docs = append(docs, part)
		}
	}

	return docs
}
