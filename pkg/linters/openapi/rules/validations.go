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
	"fmt"
	"os"

	"github.com/google/cel-go/cel"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const DeckhouseValidationsRuleName = "deckhouse-validations"

const (
	// deckhouseValidationsKey is the OpenAPI extension that carries Deckhouse CEL
	// validation rules for a module's config-values / values schema. It is the
	// dmt-side mirror of deckhouse-controller's
	// internal/packages/values/schema/cel (const ruleKey).
	deckhouseValidationsKey = "x-deckhouse-validations"
	// kubernetesValidationsKey is the upstream Kubernetes CEL-validation extension.
	// It is honored by the API server for CRDs, but Deckhouse module config-values /
	// values schemas are validated by deckhouse-controller, which only reads
	// x-deckhouse-validations. So this key in an openapi/ schema is silently ignored.
	kubernetesValidationsKey = "x-kubernetes-validations"

	// celSelfVar / celOldSelfVar are the variables deckhouse-controller exposes to
	// x-deckhouse-validations expressions; we declare the same ones so an expression
	// that compiles here compiles there.
	celSelfVar    = "self"
	celOldSelfVar = "oldSelf"
)

// DeckhouseValidationsRule validates the Deckhouse/Kubernetes CEL-validation
// extensions inside a module's openapi/ schemas.
//
// Rationale: x-deckhouse-validations is consumed only by deckhouse-controller at
// module-config validation time (see deckhouse/deckhouse
// deckhouse-controller/internal/packages/values/schema/cel). A malformed block —
// not a list, an entry missing expression/message, or an expression that is not a
// valid CEL program — is therefore never caught by the other openapi rules and
// only surfaces at runtime, when the module config is applied on a cluster. This
// rule shifts that detection left, checking the same invariants the controller
// enforces, so a broken block is reported in the module's own CI with its path.
//
// It reports two things over every openapi/ schema:
//   - x-deckhouse-validations blocks whose shape does not match what the controller
//     accepts (list of non-empty {expression, message} mappings), and expressions
//     that do not compile as CEL over self/oldSelf.
//   - x-kubernetes-validations keys, which look right but are ignored here: the
//     controller honors x-deckhouse-validations, so this is almost always a mix-up.
//
// Every finding is currently emitted at warn level; the structural/CEL checks are
// intended to become errors once modules have migrated.
type DeckhouseValidationsRule struct {
	pkg.RuleMeta
	rootPath string
}

func NewDeckhouseValidationsRule(_ *pkg.OpenAPILinterConfig, rootPath string) *DeckhouseValidationsRule {
	return &DeckhouseValidationsRule{
		RuleMeta: pkg.RuleMeta{
			Name: DeckhouseValidationsRuleName,
		},
		rootPath: rootPath,
	}
}

// Run parses an openapi/ schema and walks it for the CEL-validation extensions.
func (r *DeckhouseValidationsRule) Run(path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	shortPath := fsutils.Rel(r.rootPath, path)

	data, err := os.ReadFile(path)
	if err != nil {
		errorList.WithFilePath(shortPath).Warnf("cannot read openapi file: %s", err)

		return
	}

	// Decode like the base openapi parser (internal/openapi getFileYAMLContent).
	// A YAML syntax error is already reported by the enum/ha rules as
	// "openAPI file is not valid", so don't re-report it here — there is simply
	// nothing to walk.
	m := make(map[string]any)
	if err := yaml.UnmarshalStrict(data, &m); err != nil {
		return
	}

	r.walk(shortPath, "", m, errorList)
}

// walk descends the decoded schema and dispatches on the CEL-validation keys.
func (r *DeckhouseValidationsRule) walk(filePath, path string, node any, errorList *errors.LintRuleErrorsList) {
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			childPath := joinValidationsPath(path, key)

			switch key {
			case deckhouseValidationsKey:
				r.checkDeckhouseValidations(filePath, childPath, child, errorList)

				continue
			case kubernetesValidationsKey:
				errorList.WithFilePath(filePath).Warnf(
					"%q at %q is not honored in a module openapi schema (deckhouse-controller validates module config with %q); did you mean %q?",
					kubernetesValidationsKey, childPath, deckhouseValidationsKey, deckhouseValidationsKey)

				continue
			}

			r.walk(filePath, childPath, child, errorList)
		}
	case []any:
		for i := range v {
			r.walk(filePath, fmt.Sprintf("%s[%d]", path, i), v[i], errorList)
		}
	}
}

// checkDeckhouseValidations enforces the same shape deckhouse-controller requires
// (see cel.ValidateTransition): a non-empty list of mappings, each with a non-empty
// string expression and message, and each expression a compilable CEL program.
func (r *DeckhouseValidationsRule) checkDeckhouseValidations(filePath, path string, raw any, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(filePath)

	list, ok := raw.([]any)
	if !ok {
		errorList.Warnf("%q at %q must be a list of {expression, message} entries, got %s",
			deckhouseValidationsKey, path, yamlKind(raw))

		return
	}

	if len(list) == 0 {
		errorList.Warnf("%q at %q must contain at least one {expression, message} entry",
			deckhouseValidationsKey, path)

		return
	}

	// The controller builds its CEL env with self/oldSelf; declare the same so that
	// an expression accepted here is accepted there. self/oldSelf are dynamically
	// typed because the concrete values are unknown at lint time — this is the most
	// permissive binding and will not reject an expression the controller accepts.
	env, envErr := newValidationsCELEnv()

	for i := range list {
		entryPath := fmt.Sprintf("%s[%d]", path, i)

		entry, ok := list[i].(map[string]any)
		if !ok || len(entry) == 0 {
			errorList.Warnf("%q entry at %q must be a non-empty mapping with %q and %q fields",
				deckhouseValidationsKey, entryPath, "expression", "message")

			continue
		}

		expression, exprOK := nonEmptyStringField(entry, "expression")
		if !exprOK {
			errorList.Warnf("%q entry at %q must have a non-empty string %q field",
				deckhouseValidationsKey, entryPath, "expression")
		}

		if _, msgOK := nonEmptyStringField(entry, "message"); !msgOK {
			errorList.Warnf("%q entry at %q must have a non-empty string %q field",
				deckhouseValidationsKey, entryPath, "message")
		}

		if exprOK && envErr == nil {
			if _, issues := env.Compile(expression); issues != nil && issues.Err() != nil {
				errorList.Warnf("%q expression at %q is not a valid CEL expression: %s",
					deckhouseValidationsKey, entryPath, issues.Err())
			}
		}
	}
}

// newValidationsCELEnv mirrors the environment deckhouse-controller compiles
// x-deckhouse-validations expressions in: self and oldSelf, dynamically typed.
func newValidationsCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable(celSelfVar, cel.DynType),
		cel.Variable(celOldSelfVar, cel.DynType),
	)
}

// nonEmptyStringField reports whether key holds a non-empty string, returning it.
func nonEmptyStringField(m map[string]any, key string) (string, bool) {
	v, found := m[key]
	if !found {
		return "", false
	}

	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}

	return s, true
}

// joinValidationsPath builds a dotted location for messages (root has empty path).
func joinValidationsPath(path, key string) string {
	if path == "" {
		return key
	}

	return path + "." + key
}

// yamlKind names the decoded YAML shape of v for human-readable messages.
func yamlKind(v any) string {
	switch v.(type) {
	case map[string]any:
		return "a mapping"
	case []any:
		return "a list"
	case string:
		return "a string"
	case bool:
		return "a boolean"
	case nil:
		return "null"
	default:
		return "a scalar"
	}
}
