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

// Package schemas validates rendered Kubernetes manifests against their
// schemas. It resolves a schema for every object by its group/version/kind
// from three sources, in order of precedence:
//
//  1. CustomResourceDefinitions shipped by the module under test (its crds/
//     directory), converted from their OpenAPI v3 schema on the fly;
//  2. third-party CRD schemas from the datree/crds-catalog, compiled into the
//     dmt binary;
//  3. built-in Kubernetes type schemas, also compiled into the binary.
//
// Objects whose kind has no known schema are skipped: dmt only reports schema
// violations, never the absence of a schema.
package schemas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Store resolves and caches schemas for rendered manifests. A Store is safe for
// concurrent use. The zero value is not usable; call New.
type Store struct {
	mu sync.Mutex

	// raw holds the embedded catalog entries (entry name -> raw JSON schema)
	// that were extracted for the objects passed to Prepare.
	raw      map[string][]byte
	compiled map[string]*jsonschema.Schema

	// module holds schemas for the CRDs shipped by the module under test,
	// compiled from their OpenAPI v3 definitions. Keyed by lookup key.
	module map[string]*jsonschema.Schema
}

// New returns an empty Store. Embedded schemas are loaded lazily on the first
// Validate call.
func New() *Store {
	return &Store{
		compiled: make(map[string]*jsonschema.Schema),
		module:   make(map[string]*jsonschema.Schema),
	}
}

// Result is the outcome of validating a single object.
type Result struct {
	// Found reports whether a schema matching the object's GVK was located.
	Found bool
	// SchemaSource identifies where the matching schema came from: "module",
	// "crd" or "k8s". Empty when Found is false.
	SchemaSource string
	// Errors holds human-readable schema violations. Empty when the object is
	// valid (or when no schema was found).
	Errors []string
}

// Valid reports whether a schema was found and the object satisfied it.
func (r Result) Valid() bool {
	return r.Found && len(r.Errors) == 0
}

// Validate looks up the schema matching obj's apiVersion/kind and validates obj
// against it. When no schema is found, it returns Result{Found: false}.
func (s *Store) Validate(obj map[string]any) Result {
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)

	if apiVersion == "" || kind == "" {
		return Result{}
	}

	group, version := splitAPIVersion(apiVersion)

	sch, source := s.lookup(group, version, kind)
	if sch == nil {
		return Result{}
	}

	instance, err := normalizeInstance(obj)
	if err != nil {
		return Result{Found: true, SchemaSource: source, Errors: []string{err.Error()}}
	}

	if err := sch.Validate(instance); err != nil {
		return Result{Found: true, SchemaSource: source, Errors: formatValidationError(err)}
	}

	return Result{Found: true, SchemaSource: source}
}

// Prepare extracts, in a single streaming pass over the embedded catalog, the
// schemas needed to validate the given objects. Because the decompressed
// catalog is very large, only the entries a module actually references are
// materialized. Prepare is optional: without it, only module-provided CRD
// schemas are available. It is safe to call more than once (each call unions in
// any newly-needed entries).
func (s *Store) Prepare(objects []map[string]any) error {
	want := make(map[string]struct{})

	for _, obj := range objects {
		apiVersion, _ := obj["apiVersion"].(string)
		kind, _ := obj["kind"].(string)

		if apiVersion == "" || kind == "" {
			continue
		}

		group, version := splitAPIVersion(apiVersion)

		// A resource may be served either by a bundled CRD schema or a built-in
		// one; request both candidates and keep whichever the catalog holds.
		want[entryKey(sourceCRD, crdLookupKey(group, version, kind))] = struct{}{}
		want[entryKey(sourceK8s, k8sLookupKey(group, version, kind))] = struct{}{}
	}

	extracted, err := extractCatalog(want)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.raw == nil {
		s.raw = extracted
	} else {
		maps.Copy(s.raw, extracted)
	}

	return nil
}

// lookup returns the compiled schema for the given GVK and the name of the
// source it came from, or (nil, "") when none is known.
func (s *Store) lookup(group, version, kind string) (*jsonschema.Schema, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Module-provided CRDs win over the bundled catalog so a module's own
	// definition is authoritative for its resources.
	if sch, ok := s.module[crdLookupKey(group, version, kind)]; ok {
		return sch, "module"
	}

	// 2. Third-party CRDs from the datree catalog (keyed by full group).
	if sch := s.compiledCatalogLocked(sourceCRD, crdLookupKey(group, version, kind)); sch != nil {
		return sch, sourceCRD
	}

	// 3. Built-in Kubernetes types (keyed by the group's first DNS label).
	if sch := s.compiledCatalogLocked(sourceK8s, k8sLookupKey(group, version, kind)); sch != nil {
		return sch, sourceK8s
	}

	return nil, ""
}

// compiledCatalogLocked returns the compiled schema for a catalog entry that was
// extracted by Prepare, compiling and caching it on first access. Callers must
// hold s.mu.
func (s *Store) compiledCatalogLocked(source, lookup string) *jsonschema.Schema {
	name := entryKey(source, lookup)

	if sch, ok := s.compiled[name]; ok {
		return sch
	}

	data, ok := s.raw[name]
	if !ok {
		s.compiled[name] = nil // negative cache
		return nil
	}

	sch, err := compileSchema(name, data)
	if err != nil {
		// A malformed catalog entry must not fail linting; treat it as "no
		// schema" and remember the miss.
		s.compiled[name] = nil
		return nil
	}

	s.compiled[name] = sch

	return sch
}

// splitAPIVersion splits a Kubernetes apiVersion into its group and version.
// The core group ("v1") yields an empty group.
func splitAPIVersion(apiVersion string) (group, version string) {
	if g, v, ok := strings.Cut(apiVersion, "/"); ok {
		return g, v
	}

	return "", apiVersion
}

// crdLookupKey builds the catalog key for CRD-sourced schemas, which are keyed
// by their full API group.
func crdLookupKey(group, version, kind string) string {
	return strings.ToLower(kind) + "__" + strings.ToLower(group) + "__" + strings.ToLower(version)
}

// k8sLookupKey builds the catalog key for built-in Kubernetes schemas, which
// are keyed by the first DNS label of the API group (matching the upstream JSON
// schema file naming).
func k8sLookupKey(group, version, kind string) string {
	short := group
	if label, _, ok := strings.Cut(group, "."); ok {
		short = label
	}

	return strings.ToLower(kind) + "__" + strings.ToLower(short) + "__" + strings.ToLower(version)
}

// compileSchema compiles a single self-contained JSON schema document.
func compileSchema(url string, raw []byte) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse schema %q: %w", url, err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("add schema %q: %w", url, err)
	}

	sch, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile schema %q: %w", url, err)
	}

	return sch, nil
}

// normalizeInstance converts a decoded manifest into the canonical JSON value
// shape (map[string]any, []any, float64, string, bool, nil) that the validator
// expects. Rendered manifests carry Kubernetes-specific numeric types
// (e.g. int64) that would otherwise trip up validation.
func normalizeInstance(obj map[string]any) (any, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}

	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// formatValidationError flattens a validator error into concise, de-duplicated
// "<instance location>: <message>" lines suitable for a lint report.
func formatValidationError(err error) []string {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []string{err.Error()}
	}

	lines := make([]string, 0)
	seen := make(map[string]struct{})

	var walk func(u *jsonschema.OutputUnit)
	walk = func(u *jsonschema.OutputUnit) {
		if u.Error != nil {
			loc := u.InstanceLocation
			if loc == "" {
				loc = "/"
			}

			line := loc + ": " + u.Error.String()
			if _, dup := seen[line]; !dup {
				seen[line] = struct{}{}
				lines = append(lines, line)
			}
		}

		for i := range u.Errors {
			walk(&u.Errors[i])
		}
	}

	walk(ve.BasicOutput())

	if len(lines) == 0 {
		return []string{strings.TrimSpace(ve.Error())}
	}

	return lines
}
