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
	"path/filepath"
	"sort"

	"github.com/deckhouse/dmt/internal/schemas"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	SchemaValidationRuleName = "schema-validation"

	crdsDirName = "crds"
)

func NewSchemaValidationRule(excludeRules []pkg.KindRuleExclude) *SchemaValidationRule {
	return &SchemaValidationRule{
		RuleMeta: pkg.RuleMeta{
			Name: SchemaValidationRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

// SchemaValidationRule validates every rendered manifest against its schema.
// Schemas come from the module's own CRDs (crds/), the bundled third-party CRD
// catalog and the bundled built-in Kubernetes schemas. Resources whose kind has
// no known schema are silently skipped.
type SchemaValidationRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

func (r *SchemaValidationRule) ValidateResourceSchemas(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	storeObjects := m.GetStorage()
	if len(storeObjects) == 0 {
		return
	}

	store := schemas.New()

	// The deckhouse repository is the source of truth for CRD versions: a kind
	// defined anywhere in the repo (e.g. cert-manager's Certificate under
	// 101-cert-manager) validates against that definition instead of the lagging
	// bundled catalog. Loaded once per repo root and shared across modules.
	if root := schemas.DeckhouseRoot(m.GetPath()); root != "" {
		store.UseRepoCRDs(schemas.LoadRepoCRDs(root))
	}

	// A module's own CRDs are authoritative for the resources it defines.
	crdsDir := filepath.Join(m.GetPath(), crdsDirName)
	if err := store.LoadModuleCRDs(crdsDir); err != nil {
		errorList.WithFilePath(crdsDir).
			Warnf("failed to load module CRDs for schema validation: %s", err)
	}

	// Surface CRDs that ship null-valued keywords (e.g. an empty `maxLength:`).
	// dmt tolerates them — Kubernetes treats such optional keywords as unset — but
	// they are worth cleaning up. Reported as warnings so validation still runs.
	for _, note := range store.ModuleCRDNotes() {
		errorList.WithFilePath(crdsDir).
			Warnf("CRD %s %s/%s has a null keyword at %q — set a value or remove the key",
				note.Kind, note.Group, note.Version, note.Path)
	}

	// Collect object bodies once so the embedded catalog is streamed a single
	// time for exactly the kinds this module renders.
	bodies := make([]map[string]any, 0, len(storeObjects))
	for _, object := range storeObjects {
		bodies = append(bodies, object.Unstructured.UnstructuredContent())
	}

	if err := store.Prepare(bodies); err != nil {
		errorList.Warnf("failed to load bundled schemas for validation: %s", err)
		return
	}

	for _, object := range storeObjects {
		kind := object.Unstructured.GetKind()
		name := object.Unstructured.GetName()

		if !r.Enabled(kind, name) {
			continue
		}

		result := store.Validate(object.Unstructured.UnstructuredContent())
		if !result.Found || result.Valid() {
			continue
		}

		// Deterministic order keeps output stable across runs.
		violations := result.Errors
		sort.Strings(violations)

		for _, violation := range violations {
			errorList.WithObjectID(object.Identity()).
				WithFilePath(object.GetPath()).
				Errorf("resource does not match its %s schema: %s", result.SchemaSource, violation)
		}
	}
}
