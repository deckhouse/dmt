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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/pss"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

const speDescriptionTODO = "TODO"

// speProposal is the SecurityPolicyException `dmt lint --fix` proposes for a pod.
type speProposal struct {
	// general is the SPE the pod's common label points to (or would point to).
	general string
	// bindGeneral is true when the pod template has no common label yet.
	bindGeneral bool
	// adds are the allowances to add, by SPE name: the whole spec of a new SPE, what
	// is missing for one the module renders already.
	adds map[string]map[string]any
	// rendered are the SPEs of the pod's namespace the module renders, by name.
	rendered map[string]storage.StoreObject
	// leftover is what no SecurityPolicyException covers.
	leftover []pss.Violation
}

// proposeSPE builds the allowances covering violations of pod from its spec, checks
// them with the rego itself and returns what they could not cover.
//
// Candidates are taken generously from the spec of every container bound to an SPE
// and then pruned: an allowance (or an element of its list) whose removal fires no
// more violations than before goes. The policy parameters (allowed capabilities,
// volume types, seccomp profiles...) thus come from the rego, not from a Go copy.
func proposeSPE(ctx context.Context, pod map[string]any, violations []pss.Violation, objects map[storage.ResourceIndex]storage.StoreObject) (*speProposal, error) {
	meta := nested(pod, "metadata")
	ns, _ := meta["namespace"].(string)
	labels := nested(pod, "metadata", "labels")

	p := &speProposal{adds: map[string]map[string]any{}, rendered: map[string]storage.StoreObject{}}

	p.general, _ = labels[speRefLabel].(string)
	if p.general == "" {
		p.general, _ = meta["name"].(string)
		p.bindGeneral = true
	}

	var spes []map[string]any

	for _, o := range objects {
		if !isSPE(o) {
			continue
		}

		if o.Unstructured.GetNamespace() == ns {
			p.rendered[o.Unstructured.GetName()] = o
		}

		spes = append(spes, o.Unstructured.Object)
	}

	kinds := map[string]bool{}
	for _, v := range violations {
		kinds[v.Kind] = true
	}

	spec := nested(pod, "spec")

	// Pod fields read the common label only, container fields the container one first.
	groups := map[string][]map[string]any{}

	for _, c := range allContainers(spec) {
		name, _ := c["name"].(string)

		target, _ := labels[speContainerLabelPrefix+name].(string)
		if target == "" {
			target = p.general
		}

		groups[target] = append(groups[target], c)
	}

	candidates := map[string]map[string]any{p.general: podAllowances(spec, kinds)}
	for target, cs := range groups {
		if candidates[target] == nil {
			candidates[target] = map[string]any{}
		}

		mergeSpec(candidates[target], containerAllowances(pod, cs, kinds))
	}

	for target, c := range candidates {
		if o, ok := p.rendered[target]; ok {
			existing, _ := o.Unstructured.Object["spec"].(map[string]any)
			subtractSpec(c, existing)
		}

		if len(c) > 0 {
			p.adds[target] = c
		}
	}

	// The pod as it is once bound to the general SPE.
	bound := deepCopy(pod)
	if p.bindGeneral {
		if nested(bound, "metadata")["labels"] == nil {
			nested(bound, "metadata")["labels"] = map[string]any{}
		}

		nested(bound, "metadata", "labels")[speRefLabel] = p.general
	}

	eval := func() ([]pss.Violation, error) {
		objs := make([]map[string]any, 0, len(spes)+len(p.adds))

		for _, o := range spes {
			m, _ := o["metadata"].(map[string]any)
			if m["namespace"] == ns && p.adds[fmt.Sprint(m["name"])] != nil {
				continue
			}

			objs = append(objs, o)
		}

		for name, add := range p.adds {
			spec := map[string]any{}

			if o, ok := p.rendered[name]; ok {
				existing, _ := o.Unstructured.Object["spec"].(map[string]any)
				mergeSpec(spec, deepCopy(existing))
			}

			mergeSpec(spec, deepCopy(add))
			objs = append(objs, newSPE(name, ns, spec))
		}

		inventory, err := pss.Inventory(objs)
		if err != nil {
			return nil, err
		}

		return pss.Eval(ctx, bound, inventory)
	}

	full, err := eval()
	if err != nil {
		return nil, err
	}

	limit := kindCounts(full)
	covered := func() bool {
		v, err := eval()
		if err != nil {
			return false
		}

		for kind, n := range kindCounts(v) {
			if n > limit[kind] {
				return false
			}
		}

		return true
	}

	for _, name := range slices.Sorted(maps.Keys(p.adds)) {
		pruneSpec(p.adds[name], covered)

		if len(p.adds[name]) == 0 {
			delete(p.adds, name)
		}
	}

	if p.leftover, err = eval(); err != nil {
		return nil, err
	}

	paths, err := pss.SPEAllowancePaths()
	if err != nil {
		return nil, err
	}

	for name, add := range p.adds {
		describeTODO(add, paths)

		problems, err := pss.ValidateSPE(newSPE(name, ns, add))
		if err != nil {
			return nil, err
		}

		if len(problems) > 0 {
			return nil, fmt.Errorf("generated SecurityPolicyException %s does not pass the CRD schema: %s", name, strings.Join(problems, "; "))
		}
	}

	return p, nil
}

// podAllowances are the candidate pod-level allowances (common label only).
func podAllowances(spec map[string]any, kinds map[string]bool) map[string]any {
	res := map[string]any{}

	if kinds[hostNetworkKind] {
		if spec["hostNetwork"] == true {
			setPath(res, true, "network", "hostNetwork", "allowedValue")
		}

		// Once an SPE lists host ports, only the listed {port, protocol} pass.
		hps := hostPorts(map[string]any{"spec": spec})

		ports := make([]any, 0, len(hps))
		for _, p := range hps {
			ports = append(ports, map[string]any{"port": p.Port, "protocol": p.Protocol})
		}

		if len(ports) > 0 {
			setPath(res, ports, "network", "hostPorts")
		}
	}

	if kinds["D8HostProcesses"] {
		for _, f := range []string{"hostPID", "hostIPC"} {
			if spec[f] == true {
				setPath(res, true, "network", f, "allowedValue")
			}
		}
	}

	volumes, _ := spec["volumes"].([]any)

	if kinds["D8AllowedVolumeTypes"] || kinds["D8AllowedHostPaths"] {
		var types []any

		for _, v := range volumes {
			for k := range mapOf(v) {
				if k != "name" && !slices.Contains(types, any(k)) {
					types = append(types, k)
				}
			}
		}

		if len(types) > 0 {
			slices.SortFunc(types, func(a, b any) int { return strings.Compare(a.(string), b.(string)) })
			setPath(res, types, "volumes", "types", "allowedValues")
		}
	}

	if kinds["D8AllowedHostPaths"] {
		var paths []any

		for _, v := range volumes {
			vm := mapOf(v)

			path, _ := mapOf(vm["hostPath"])["path"].(string)
			if path == "" {
				continue
			}

			name, _ := vm["name"].(string)

			// A volume mounted both read-only and read-write breaks the rego
			// (eval_conflict_error) once an SPE names its path: left uncovered.
			if ro, ok := mountReadOnly(spec, name); ok {
				paths = append(paths, map[string]any{"path": path, "readOnly": ro})
			}
		}

		if len(paths) > 0 {
			setPath(res, paths, "volumes", "hostPath", "allowedValues")
		}
	}

	return res
}

// mountReadOnly returns readOnly of the mounts of volume as the rego sees it: absent
// is false, and so is an unmounted volume. The second result is false when mounts disagree.
func mountReadOnly(spec map[string]any, volume string) (bool, bool) {
	seen := map[bool]bool{}

	for _, c := range allContainers(spec) {
		mounts, _ := c["volumeMounts"].([]any)
		for _, m := range mounts {
			mm := mapOf(m)
			if mm["name"] == volume {
				ro, _ := mm["readOnly"].(bool)
				seen[ro] = true
			}
		}
	}

	return seen[true], len(seen) < 2
}

// containerAllowances are the candidate container-level allowances of containers cs
// of pod, all bound to one SPE.
func containerAllowances(pod map[string]any, cs []map[string]any, kinds map[string]bool) map[string]any {
	spec := nested(pod, "spec")
	annotations := nested(pod, "metadata", "annotations")
	res := map[string]any{}

	sc := func(c map[string]any, f string) any { return nested(c, "securityContext")[f] }

	// effective: the container's securityContext, then the pod's
	effective := func(c map[string]any, f string) any {
		if v, ok := nested(c, "securityContext")[f]; ok {
			return v
		}

		return nested(spec, "securityContext")[f]
	}

	union := func(path []string, v any) {
		if v == nil || v == "" {
			return
		}

		list, _ := getPath(res, path...).([]any)
		if !slices.ContainsFunc(list, func(e any) bool { return sameValue(e, v) }) {
			setPath(res, append(list, v), path...)
		}
	}

	var dropIntersection []any

	dropNeeded := false

	for _, c := range cs {
		name, _ := c["name"].(string)

		if kinds["D8PrivilegedContainer"] && sc(c, "privileged") == true {
			setPath(res, true, "securityContext", "privileged", "allowedValue")
		}

		// unset is true
		if kinds["D8AllowPrivilegeEscalation"] && sc(c, "allowPrivilegeEscalation") != false {
			setPath(res, true, "securityContext", "allowPrivilegeEscalation", "allowedValue")
		}

		if kinds["D8AllowedCapabilities"] {
			caps := nested(c, "securityContext", "capabilities")

			add, _ := caps["add"].([]any)
			for _, a := range add {
				union([]string{"securityContext", "capabilities", "allowedValues", "add"}, a)
			}

			// The SPE drop must be a non-empty subset of the drop of every container
			// that does not drop ALL; an empty drop no SPE covers.
			drop, _ := caps["drop"].([]any)
			if len(drop) > 0 && !slices.ContainsFunc(drop, func(d any) bool { return strings.EqualFold(fmt.Sprint(d), "ALL") }) {
				if !dropNeeded {
					dropNeeded, dropIntersection = true, slices.Clone(drop)
				} else {
					dropIntersection = slices.DeleteFunc(dropIntersection, func(d any) bool {
						return !slices.ContainsFunc(drop, func(e any) bool { return strings.EqualFold(fmt.Sprint(d), fmt.Sprint(e)) })
					})
				}
			}
		}

		if kinds["D8AllowedUsers"] {
			if user := effective(c, "runAsUser"); user != nil {
				union([]string{"securityContext", "runAsUser", "allowedValues"}, toInt64(user))

				// one value for all containers: the root one wins
				nonRoot, _ := effective(c, "runAsNonRoot").(bool)
				if getPath(res, "securityContext", "runAsNonRoot") == nil || toInt64(user) == 0 {
					setPath(res, nonRoot, "securityContext", "runAsNonRoot", "allowedValue")
				}
			}
		}

		if kinds["D8AllowedSeccompProfiles"] {
			union([]string{"securityContext", "seccompProfile", "allowedValues"}, seccompProfile(spec, annotations, c, name))
		}

		if kinds["D8AppArmor"] {
			union([]string{"securityContext", "appArmorProfile", "allowedValues"}, appArmorProfile(spec, annotations, c, name))
		}

		if kinds["D8AllowedProcMount"] {
			union([]string{"securityContext", "procMount", "allowedValues"}, sc(c, "procMount"))
		}
	}

	if dropNeeded && len(dropIntersection) > 0 {
		setPath(res, dropIntersection, "securityContext", "capabilities", "allowedValues", "drop")
	}

	return res
}

// seccompProfile is the profile as D8AllowedSeccompProfiles resolves it: container
// field, pod field, container annotation, pod annotation; the raw value, an SPE gets
// no synonyms. Localhost becomes localhost/<localhostProfile>.
func seccompProfile(spec, annotations, c map[string]any, name string) any {
	for _, v := range []any{
		nested(c, "securityContext", "seccompProfile")["type"],
		nested(spec, "securityContext", "seccompProfile")["type"],
		annotations["container.seccomp.security.alpha.kubernetes.io/"+name],
		annotations["seccomp.security.alpha.kubernetes.io/pod"],
	} {
		if v == nil {
			continue
		}

		if v != "Localhost" {
			return v
		}

		for _, lp := range []any{
			nested(c, "securityContext", "seccompProfile")["localhostProfile"],
			nested(spec, "securityContext", "seccompProfile")["localhostProfile"],
		} {
			if lp != nil {
				return fmt.Sprintf("localhost/%v", lp)
			}
		}

		return "localhost"
	}

	return nil
}

// appArmorProfile is the profile as D8AppArmor resolves it: container field, container
// annotation, pod field; the rego compares the value normalized to the annotation
// form, which is also the only form the CRD accepts. Localhost from a field has no
// such form (upstream bug): no SPE covers it.
func appArmorProfile(spec, annotations, c map[string]any, name string) any {
	for _, v := range []any{
		nested(c, "securityContext", "appArmorProfile")["type"],
		annotations["container.apparmor.security.beta.kubernetes.io/"+name],
		nested(spec, "securityContext", "appArmorProfile")["type"],
	} {
		switch v {
		case nil:
			continue
		case "RuntimeDefault":
			return "runtime/default"
		case "Unconfined":
			return "unconfined"
		case "Localhost":
			return nil
		}

		return v
	}

	return nil
}

// pruneSpec removes from node every key and list element covered() does without.
// allowedValue(s) are never removed alone: without them their allowance is. Elements
// of capabilities drop are kept: it lists what the container must drop, fewer is laxer.
func pruneSpec(node map[string]any, covered func() bool) {
	for _, k := range slices.Sorted(maps.Keys(node)) {
		v := node[k]

		if k != "allowedValue" && k != "allowedValues" {
			delete(node, k)

			if covered() {
				continue
			}

			node[k] = v
		}

		switch vv := v.(type) {
		case map[string]any:
			pruneSpec(vv, covered)
		case []any:
			for i := 0; i < len(vv) && k != "drop"; {
				node[k] = slices.Delete(slices.Clone(vv), i, i+1)
				if covered() {
					vv = node[k].([]any)

					continue
				}

				node[k] = vv
				i++
			}
		}
	}
}

// describeTODO puts metadata.description: TODO on every allowance of spec.
func describeTODO(spec map[string]any, paths [][]string) {
	var walk func(node any, tpl []string)

	walk = func(node any, tpl []string) {
		switch {
		case len(tpl) == 0:
			if m, ok := node.(map[string]any); ok && m["metadata"] == nil {
				m["metadata"] = map[string]any{"description": speDescriptionTODO}
			}
		case tpl[0] == pss.ArrayItem:
			list, _ := node.([]any)
			for _, item := range list {
				walk(item, tpl[1:])
			}
		default:
			if child := mapOf(node)[tpl[0]]; child != nil {
				walk(child, tpl[1:])
			}
		}
	}

	for _, p := range paths {
		walk(spec, p)
	}
}

// mergeSpec merges src into dst: maps recursively, lists by appending missing
// elements, scalars overwrite.
func mergeSpec(dst, src map[string]any) {
	for k, v := range src {
		switch vv := v.(type) {
		case map[string]any:
			d, ok := dst[k].(map[string]any)
			if !ok {
				d = map[string]any{}
				dst[k] = d
			}

			mergeSpec(d, vv)
		case []any:
			list, _ := dst[k].([]any)
			for _, e := range vv {
				if !slices.ContainsFunc(list, func(x any) bool { return sameValue(x, e) }) {
					list = append(list, e)
				}
			}

			dst[k] = list
		default:
			dst[k] = v
		}
	}
}

// subtractSpec removes from spec what existing already has, leaving the additions.
func subtractSpec(spec, existing map[string]any) {
	for k, v := range spec {
		switch vv := v.(type) {
		case map[string]any:
			subtractSpec(vv, mapOf(existing[k]))

			if len(vv) == 0 {
				delete(spec, k)
			}
		case []any:
			have, _ := existing[k].([]any)

			vv = slices.DeleteFunc(vv, func(e any) bool {
				return slices.ContainsFunc(have, func(x any) bool { return sameValue(x, e) })
			})
			if len(vv) == 0 {
				delete(spec, k)
			} else {
				spec[k] = vv
			}
		default:
			if sameValue(existing[k], v) {
				delete(spec, k)
			}
		}
	}
}

// sameValue compares values of unstructured objects whatever their number types,
// list elements without their metadata (a description).
func sameValue(a, b any) bool {
	strip := func(v any) []byte {
		if m, ok := v.(map[string]any); ok {
			c := maps.Clone(m)
			delete(c, "metadata")
			v = c
		}

		data, _ := json.Marshal(v)

		return data
	}

	return bytes.Equal(strip(a), strip(b))
}

func kindCounts(vs []pss.Violation) map[string]int {
	res := map[string]int{}
	for _, v := range vs {
		res[v.Kind]++
	}

	return res
}

func newSPE(name, ns string, spec map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": pss.SPEAPIVersion,
		"kind":       pss.SPEKind,
		"metadata":   map[string]any{"name": name, "namespace": ns},
		"spec":       spec,
	}
}

func allContainers(spec map[string]any) []map[string]any {
	lists := make([][]any, 0, 3)
	n := 0

	for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
		list, _ := spec[field].([]any)
		lists = append(lists, list)
		n += len(list)
	}

	res := make([]map[string]any, 0, n)

	for _, list := range lists {
		for _, c := range list {
			res = append(res, mapOf(c))
		}
	}

	return res
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func getPath(m map[string]any, path ...string) any {
	for _, p := range path[:len(path)-1] {
		m = mapOf(m[p])
	}

	return m[path[len(path)-1]]
}

func setPath(m map[string]any, v any, path ...string) {
	for _, p := range path[:len(path)-1] {
		next, ok := m[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[p] = next
		}

		m = next
	}

	m[path[len(path)-1]] = v
}

func deepCopy(m map[string]any) map[string]any {
	data, _ := json.Marshal(m)

	var res map[string]any

	_ = json.Unmarshal(data, &res)

	return res
}

// speFix is the autofix of a pod-security-standards finding: it writes the proposed
// SecurityPolicyExceptions the module does not render yet as new templates and
// returns, as a note, what is left for a human: binding the pod to the SPE (the pod
// template is a helm template dmt does not edit), additions to SPEs the module has
// already, descriptions in place of TODO and violations no SPE covers.
//
// The proposal is made here, while the module's objects are alive: fixes run after
// the module is linted and its storage released.
func (r *PodSecurityStandardsRule) speFix(ctx context.Context, object storage.StoreObject, pod map[string]any,
	violations []pss.Violation, objects map[storage.ResourceIndex]storage.StoreObject) errors.AutofixFunc {
	p, err := proposeSPE(ctx, pod, violations, objects)
	if err != nil {
		return func() error { return fmt.Errorf("cannot propose a SecurityPolicyException: %w", err) }
	}

	workloads := workloadsIn(objects, filepath.Dir(object.AbsPath))
	modulePath := r.module.GetPath()
	leftover, _ := violationLines(pod, p.leftover)

	return func() error {
		var notes []string

		for _, name := range slices.Sorted(maps.Keys(p.adds)) {
			note, err := writeSPE(modulePath, object, workloads, p, name)
			if err != nil {
				return err
			}

			notes = append(notes, note)
		}

		_, rendered := p.rendered[p.general]
		if p.bindGeneral && (p.adds[p.general] != nil || rendered) {
			path := "metadata.labels"
			if tp, ok := podTemplatePaths[object.Unstructured.GetKind()]; ok {
				path = strings.Join(tp, ".") + ".metadata.labels"
			}

			notes = append(notes, fmt.Sprintf("Bind the pod to SecurityPolicyException %s: add the label %q to %s of %s/%s in %s.", p.general,
				speRefLabel+": "+p.general, path, object.Unstructured.GetKind(), object.Unstructured.GetName(), object.ShortPath()))
		}

		if len(p.adds) > 0 {
			notes = append(notes, "Replace every description: TODO with the reason the component needs the allowance.")
		}

		if len(leftover) > 0 {
			notes = append(notes, "No SecurityPolicyException covers the rest, fix the pod spec:\n"+strings.Join(leftover, "\n"))
		}

		return errors.FixNote(strings.Join(notes, "\n"))
	}
}

// writeSPE writes the new SPE name as a template next to the object's one, or, for
// one the module renders already or a file that exists, tells what to add.
func writeSPE(modulePath string, object storage.StoreObject, workloads int, p *speProposal, name string) (string, error) {
	spec, err := yaml.Marshal(map[string]any{"spec": p.adds[name]})
	if err != nil {
		return "", err
	}

	if o, ok := p.rendered[name]; ok {
		return fmt.Sprintf("Add to SecurityPolicyException %s in %s:\n%s", name, o.ShortPath(), bytes.TrimSpace(spec)), nil
	}

	dir := filepath.Dir(object.AbsPath)

	templates := filepath.Join(modulePath, "templates")
	if rel, err := filepath.Rel(templates, dir); err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("the template of %s is outside %s, not writing a SecurityPolicyException next to it", object.Identity(), templates)
	}

	// The convention is security-policy-exception.yaml in the component directory;
	// a directory with templates of several pods gets one file per SPE.
	file := "security-policy-exception-" + name + ".yaml"
	if name == object.Unstructured.GetName() && workloads == 1 {
		file = "security-policy-exception.yaml"
	}

	path := filepath.Join(dir, file)
	rel, _ := filepath.Rel(modulePath, path)
	content := speTemplate(modulePath, name, object.Unstructured.GetNamespace(), spec)

	written := fmt.Sprintf("Generated SecurityPolicyException %s in %s.", name, rel)

	if data, err := os.ReadFile(path); err == nil {
		// another render variant of the same module wrote it in this run
		if string(data) == content {
			return written, nil
		}

		return fmt.Sprintf("%s exists, not overwritten; SecurityPolicyException %s to add:\n%s", rel, name, content), nil
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // a helm template, as the rest
		return "", err
	}

	return written, nil
}

func speTemplate(modulePath, name, ns string, spec []byte) string {
	// The labels object-recommended-labels requires: by lib-helm where the module has it.
	labels := "  labels:\n    heritage: deckhouse\n    module: {{ .Chart.Name }}\n"
	if _, err := os.Stat(filepath.Join(modulePath, "charts", "helm_lib")); err == nil {
		labels = fmt.Sprintf("  {{- include \"helm_lib_module_labels\" (list . (dict \"app\" %q)) | nindent 2 }}\n", name)
	}

	return fmt.Sprintf(`{{- if .Values.global.enabledModules | has "admission-policy-engine" }}
---
apiVersion: %s
kind: %s
metadata:
  name: %s
  namespace: %s
%s%s{{- end }}
`, pss.SPEAPIVersion, pss.SPEKind, name, ns, labels, spec)
}

// workloadsIn counts the objects creating pods whose templates are in dir.
func workloadsIn(objects map[storage.ResourceIndex]storage.StoreObject, dir string) int {
	n := 0

	for _, o := range objects {
		if podOf(o) != nil && filepath.Dir(o.AbsPath) == dir {
			n++
		}
	}

	return n
}
