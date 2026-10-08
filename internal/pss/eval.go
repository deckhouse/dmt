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

package pss

import (
	"context"
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

const (
	speAPIVersion = "deckhouse.io/v1alpha1"
	speKind       = "SecurityPolicyException"
)

// Violation is one entry of a constraint's `violation` set.
type Violation struct {
	Standard string
	Kind     string
	Msg      string
	Details  any // "details" of the violation as rego returned it, nil if absent
}

type query struct {
	check *Check
	pq    rego.PreparedEvalQuery
}

var queries = sync.OnceValues(prepare)

// prepare compiles every Check into its own query, once per process: the
// templates share rego packages, and in one compiler their `violation` rules
// would merge. Gatekeeper isolates templates the same way.
//
// data.inventory differs per module, so it is not put into the store bound at
// prepare time but passed with the input and swapped in by `with`: the prepared
// queries stay shared across modules linted in parallel.
func prepare() ([]query, error) {
	checks, err := Checks()
	if err != nil {
		return nil, err
	}

	res := make([]query, 0, len(checks))

	for i := range checks {
		c := &checks[i]
		name := c.Standard + "/" + c.Kind

		mod, err := ast.ParseModule(name+".rego", c.Rego)
		if err != nil {
			return nil, fmt.Errorf("pss %s: parse rego: %w", name, err)
		}

		opts := []func(*rego.Rego){
			rego.Query(mod.Package.Path.String() + ".violation with data.inventory as input.inventory"),
			rego.Module(name+".rego", c.Rego),
		}
		for j, lib := range c.Libs {
			opts = append(opts, rego.Module(fmt.Sprintf("%s/lib%d.rego", name, j), lib))
		}

		pq, err := rego.New(opts...).PrepareForEval(context.Background())
		if err != nil {
			return nil, fmt.Errorf("pss %s: prepare: %w", name, err)
		}

		res = append(res, query{check: c, pq: pq})
	}

	return res, nil
}

// Inventory builds gatekeeper's data.inventory from SecurityPolicyException objects:
// inventory.namespace[ns]["deckhouse.io/v1alpha1"].SecurityPolicyException[name].
// Objects of other kinds are ignored.
func Inventory(objects []map[string]any) map[string]any {
	byNS := map[string]any{}

	for _, o := range objects {
		if o["apiVersion"] != speAPIVersion || o["kind"] != speKind {
			continue
		}

		meta, _ := o["metadata"].(map[string]any)
		ns, _ := meta["namespace"].(string)
		name, _ := meta["name"].(string)

		nsMap, ok := byNS[ns].(map[string]any)
		if !ok {
			nsMap = map[string]any{speAPIVersion: map[string]any{speKind: map[string]any{}}}
			byNS[ns] = nsMap
		}

		nsMap[speAPIVersion].(map[string]any)[speKind].(map[string]any)[name] = o
	}

	return map[string]any{"namespace": byNS}
}

// Eval runs every Check of both standards against pod, a `kind: Pod` object, with
// inventory from Inventory. An error means the pod was not checked, never that it passed.
func Eval(ctx context.Context, pod, inventory map[string]any) ([]Violation, error) {
	qs, err := queries()
	if err != nil {
		return nil, err
	}

	meta, _ := pod["metadata"].(map[string]any)

	// Only object and operation are read by the synced rego; the rest mirrors
	// gatekeeper's AdmissionRequest for a Pod create.
	review := map[string]any{
		"kind":      map[string]any{"group": "", "version": "v1", "kind": "Pod"},
		"operation": "CREATE",
		"name":      meta["name"],
		"namespace": meta["namespace"],
		"object":    pod,
	}

	var res []Violation

	for _, q := range qs {
		rs, err := q.pq.Eval(ctx, rego.EvalInput(map[string]any{
			"review":     review,
			"parameters": q.check.Parameters,
			"inventory":  inventory,
		}))
		if err != nil {
			return nil, fmt.Errorf("pss %s/%s: %w", q.check.Standard, q.check.Kind, err)
		}

		if len(rs) != 1 || len(rs[0].Expressions) != 1 {
			return nil, fmt.Errorf("pss %s/%s: unexpected result %v", q.check.Standard, q.check.Kind, rs)
		}

		set, ok := rs[0].Expressions[0].Value.([]any)
		if !ok {
			return nil, fmt.Errorf("pss %s/%s: violation is %T, not a set", q.check.Standard, q.check.Kind, rs[0].Expressions[0].Value)
		}

		for _, v := range set {
			m, _ := v.(map[string]any)
			msg, _ := m["msg"].(string)
			res = append(res, Violation{Standard: q.check.Standard, Kind: q.check.Kind, Msg: msg, Details: m["details"]})
		}
	}

	return res, nil
}
