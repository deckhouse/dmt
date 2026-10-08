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
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/flags"
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

	inventory, err := pss.Inventory(all)
	if err != nil {
		r.errorList.Errorf("Cannot check Pod Security Standards: %v", err)

		return
	}

	for _, o := range objects {
		r.checkObject(ctx, o, inventory, objects)
	}
}

func (r *PodSecurityStandardsRule) checkObject(ctx context.Context, object storage.StoreObject, inventory map[string]any,
	objects map[storage.ResourceIndex]storage.StoreObject) {
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

	lines, unexceptable := violationLines(violations)

	advice := "Fix the pod spec, or, if the deviation is really needed, describe it in a SecurityPolicyException: " + pssDocURL
	if unexceptable {
		advice = "Fix the pod spec: no SecurityPolicyException can cover these violations"
	} else if flags.Fix {
		errorList = errorList.WithFix(r.speFix(ctx, object, pod, violations, objects))
	}

	errorList.Errorf("%s/%s violates Pod Security Standards (restricted) and no SecurityPolicyException covers it:\n%s\n%s",
		object.Unstructured.GetKind(), object.Unstructured.GetName(), strings.Join(lines, "\n"), advice)
}

// violationLines lists violations as sorted "- <Kind>: <msg>" lines, a msg fired by
// both standards once. The container of container checks is in their msg already,
// and D8HostNetwork lists hostNetwork and every disallowed port in its one msg.
//
// Violations no SecurityPolicyException can cover get a line with their containers
// (taken from details). unexceptable is true when every violation is such.
func violationLines(violations []pss.Violation) ([]string, bool) {
	seen := map[string]bool{}
	containers := map[string][]string{}
	unexceptable := true

	var res []string

	for _, v := range violations {
		line := "- " + v.Kind + ": " + v.Msg
		if !seen[line] {
			seen[line] = true
			res = append(res, line)
		}

		u, ok := unexceptableKinds[v.Kind]
		d, _ := v.Details.(map[string]any)

		if !ok || !u.match(d) {
			unexceptable = false

			continue
		}

		name, _ := d["container"].(string)
		if !slices.Contains(containers[v.Kind], name) {
			containers[v.Kind] = append(containers[v.Kind], name)
		}
	}

	for kind, names := range containers {
		slices.Sort(names)
		res = append(res, "- "+kind+": "+unexceptableKinds[kind].msg+" (computed by dmt): "+strings.Join(names, ", "))
	}

	slices.Sort(res)

	return res, unexceptable
}

const hostNetworkKind = "D8HostNetwork"

// unexceptableKinds are violations the rego gives no SecurityPolicyException a way
// to cover, matched by their details: an SPE's capabilities.drop must be a non-empty
// subset of the container's, and a container with neither runAsUser nor
// runAsNonRoot: true is denied with no SPE lookup at all (the only D8AllowedUsers
// violation whose details carry runAsNonRoot).
var unexceptableKinds = map[string]struct {
	msg   string
	match func(details map[string]any) bool
}{
	"D8AllowedCapabilities": {
		msg: "no SecurityPolicyException can cover an empty capabilities.drop, add drop: [ALL] to containers",
		match: func(d map[string]any) bool {
			actual, _ := d["actual"].([]any)
			return d["field"] == "capabilities.drop" && len(actual) == 0
		},
	},
	"D8AllowedUsers": {
		msg: "no SecurityPolicyException can cover unset runAsUser and runAsNonRoot, set runAsUser for containers",
		match: func(d map[string]any) bool {
			_, ok := d["runAsNonRoot"]
			return ok
		},
	},
}

type hostPort struct {
	Port     int64
	Protocol string
}

func (p hostPort) String() string { return fmt.Sprintf("%d/%s", p.Port, p.Protocol) }

// hostPorts returns sorted unique host ports of the pod's containers, init and
// ephemeral containers: every hostPort, with hostNetwork every containerPort too.
// The protocol defaults to TCP.
func hostPorts(pod map[string]any) []hostPort {
	spec := nested(pod, "spec")
	hostNetwork, _ := spec["hostNetwork"].(bool)

	set := map[hostPort]bool{}

	for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
		list, _ := spec[field].([]any)
		for _, c := range list {
			cm, _ := c.(map[string]any)

			ports, _ := cm["ports"].([]any)
			for _, p := range ports {
				port, _ := p.(map[string]any)

				proto, _ := port["protocol"].(string)
				if proto == "" {
					proto = "TCP"
				}

				keys := []string{"hostPort"}
				if hostNetwork {
					keys = append(keys, "containerPort")
				}

				for _, k := range keys {
					if n := toInt64(port[k]); n != 0 {
						set[hostPort{Port: n, Protocol: proto}] = true
					}
				}
			}
		}
	}

	res := slices.Collect(maps.Keys(set))
	slices.SortFunc(res, func(a, b hostPort) int {
		return cmp.Or(cmp.Compare(a.Port, b.Port), strings.Compare(a.Protocol, b.Protocol))
	})

	return res
}

// toInt64 reads a number of an unstructured object: int64 from the renderer,
// float64 from plain JSON.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}

	return 0
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
