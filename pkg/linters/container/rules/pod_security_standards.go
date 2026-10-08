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
	"strings"

	"github.com/deckhouse/dmt/internal/pss"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PodSecurityStandardsRuleName = "pod-security-standards"

	pssDocURL = "https://deckhouse.ru/modules/admission-policy-engine/latest/#исключения-из-политик-безопасности"
)

// podTemplatePaths locates the pod template of the controllers admission-policy-engine
// checks in system namespaces (workload_kinds). Pod is checked as is.
var podTemplatePaths = map[string][]string{
	"Deployment":            {"spec", "template"},
	"StatefulSet":           {"spec", "template"},
	"DaemonSet":             {"spec", "template"},
	"ReplicationController": {"spec", "template"},
	"Job":                   {"spec", "template"},
	"CronJob":               {"spec", "jobTemplate", "spec", "template"},
}

func NewPodSecurityStandardsRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *PodSecurityStandardsRule {
	return &PodSecurityStandardsRule{
		RuleMeta: pkg.RuleMeta{
			Name: PodSecurityStandardsRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(PodSecurityStandardsRuleName),
	}
}

// PodSecurityStandardsRule runs the PSS rego of admission-policy-engine (baseline +
// restricted, see internal/pss) against workloads in system namespaces, taking the
// module's rendered SecurityPolicyExceptions into account. Since DKP 1.79 a violation
// there is denied.
type PodSecurityStandardsRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*PodSecurityStandardsRule)(nil)

func (r *PodSecurityStandardsRule) Check(ctx context.Context) {
	objects := r.module.GetStorage()

	all := make([]map[string]any, 0, len(objects))
	for _, o := range objects {
		all = append(all, o.Unstructured.Object)
	}

	inventory := pss.Inventory(all)

	for _, o := range objects {
		r.checkObject(ctx, o, inventory)
	}
}

func (r *PodSecurityStandardsRule) checkObject(ctx context.Context, object storage.StoreObject, inventory map[string]any) {
	ns := object.Unstructured.GetNamespace()
	if !strings.HasPrefix(ns, "d8-") && !strings.HasPrefix(ns, "kube-") {
		return
	}

	pod := podOf(object)
	if pod == nil {
		return
	}

	errorList := r.errorList.WithFilePath(object.GetPath()).WithObjectID(object.Identity())

	violations, err := pss.Eval(ctx, pod, inventory)
	if err != nil {
		errorList.Errorf("Cannot check Pod Security Standards: %v", err)

		return
	}

	if len(violations) == 0 {
		return
	}

	errorList.Errorf("%s/%s violates Pod Security Standards (restricted) and no SecurityPolicyException covers it. "+
		"Fix the pod spec, or, if the deviation is really needed, describe it in a SecurityPolicyException: %s",
		object.Unstructured.GetKind(), object.Unstructured.GetName(), pssDocURL)
}

// podOf returns the Pod the object creates: the object itself for a Pod, a `kind: Pod`
// built from the pod template for a controller (metadata from the template, name and
// namespace from the controller), nil for any other kind.
//
// Controllers are checked as the Pods they create on purpose: the rego is lenient to a
// controller whose template lacks runAsUser/runAsNonRoot (a mutator might set it), and
// dmt sees nothing but templates.
func podOf(object storage.StoreObject) map[string]any {
	obj := object.Unstructured.Object
	kind := object.Unstructured.GetKind()

	if kind == "Pod" {
		return obj
	}

	path, ok := podTemplatePaths[kind]
	if !ok {
		return nil
	}

	tmpl := nested(obj, path...)

	// A fresh metadata map: storage objects are shared with other rules.
	meta := map[string]any{}
	for k, v := range nested(tmpl, "metadata") {
		meta[k] = v
	}

	meta["name"] = object.Unstructured.GetName()
	meta["namespace"] = object.Unstructured.GetNamespace()

	pod := map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": meta}
	if spec, ok := tmpl["spec"]; ok {
		pod["spec"] = spec
	}

	return pod
}

func nested(m map[string]any, path ...string) map[string]any {
	for _, p := range path {
		m, _ = m[p].(map[string]any)
	}

	return m
}
