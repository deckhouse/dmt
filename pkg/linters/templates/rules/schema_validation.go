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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const SchemaValidationRuleName = "schema-validation"

func NewSchemaValidationRule(excludeRules []pkg.KindRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *SchemaValidationRule {
	return &SchemaValidationRule{
		RuleMeta: pkg.RuleMeta{
			Name: SchemaValidationRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(SchemaValidationRuleName),
	}
}

// SchemaValidationRule decodes every rendered manifest into the Go type that
// serves it, strictly: a field of the wrong type and a field the API does not
// declare are both errors. The types come from the k8s.io/api version dmt is
// built against, so what the rule checks against is whatever Kubernetes release
// that is — there is no schema snapshot to keep in sync.
//
// Only standard Kubernetes resources are checked. A custom resource has no Go
// type registered and is skipped, as is any other unregistered kind: the rule
// reports violations, never the absence of a type.
type SchemaValidationRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*SchemaValidationRule)(nil)

func (r *SchemaValidationRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		kind := object.Unstructured.GetKind()

		if !r.Enabled(kind, object.Unstructured.GetName()) {
			continue
		}

		// New reports an error for a kind the scheme does not know, which is how a
		// custom resource — anything served by a CRD rather than by the API server
		// itself — is left alone.
		typed, err := scheme.Scheme.New(object.Unstructured.GroupVersionKind())
		if err != nil {
			continue
		}

		err = runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(
			object.Unstructured.UnstructuredContent(), typed, true)
		if err == nil {
			continue
		}

		for _, violation := range violations(err) {
			r.errorList.WithObjectID(object.Identity()).
				WithFilePath(object.GetPath()).
				Errorf("resource does not match the %s API: %s", kind, violation)
		}
	}
}

// violations flattens a decode failure into one message per problem. Unknown
// fields arrive as a strict-decoding error carrying one entry per field, already
// ordered; anything else is a type mismatch, which aborts the decode and so is
// always a single message.
func violations(err error) []string {
	strict, ok := runtime.AsStrictDecodingError(err)
	if !ok {
		return []string{err.Error()}
	}

	unknown := strict.Errors()
	out := make([]string, 0, len(unknown))

	for _, e := range unknown {
		out = append(out, e.Error())
	}

	return out
}
