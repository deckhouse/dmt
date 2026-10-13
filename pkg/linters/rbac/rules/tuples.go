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
	"slices"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// A tuple is the atom the sync rule compares: one (apiGroup, resource, resourceName, verb) a
// PolicyRule grants, or one (url, verb) for a non-resource rule. Wildcards are compared literally;
// whether they are acceptable is the wildcards rule's business.
type tuple string

func resourceTuple(group, resource, name, verb string) tuple {
	return tuple(group + "|" + resource + "|" + name + "|" + verb)
}

func urlTuple(url, verb string) tuple {
	return tuple("url:" + url + "|" + verb)
}

// String renders the tuple for a finding.
func (t tuple) String() string {
	s := string(t)
	if rest, ok := strings.CutPrefix(s, "url:"); ok {
		url, verb, _ := strings.Cut(rest, "|")
		return verb + " " + url
	}

	parts := strings.SplitN(s, "|", 4)
	group, resource, name, verb := parts[0], parts[1], parts[2], parts[3]

	if group == "" {
		group = `""`
	}

	out := verb + " " + group + "/" + resource
	if name != "" {
		out += " (" + name + ")"
	}

	return out
}

type tupleSet map[tuple]struct{}

func (s tupleSet) add(t tuple) { s[t] = struct{}{} }

// uncoveredBy returns the tuples of s that other does not grant, sorted. A tuple limited to one
// object name is granted by the same group, resource and verb without a name as well: a rule on
// every ModuleConfig covers the one on the module's own.
func (s tupleSet) uncoveredBy(other tupleSet) []tuple {
	var out []tuple

	for t := range s {
		if _, ok := other[t]; ok {
			continue
		}

		if parts := strings.Split(string(t), "|"); len(parts) == 4 && parts[2] != "" {
			if _, ok := other[resourceTuple(parts[0], parts[1], "", parts[3])]; ok {
				continue
			}
		}

		out = append(out, t)
	}

	slices.Sort(out)

	return out
}

// minus returns the tuples of s absent from other, sorted.
func (s tupleSet) minus(other tupleSet) []tuple {
	var out []tuple

	for t := range s {
		if _, ok := other[t]; !ok {
			out = append(out, t)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out
}

// expandPolicyRule turns one raw rule into its tuples.
func expandPolicyRule(rule rbacyaml.PolicyRule, into tupleSet) {
	if len(rule.NonResourceURLs) > 0 {
		for _, url := range rule.NonResourceURLs {
			for _, verb := range rule.Verbs {
				into.add(urlTuple(url, verb))
			}
		}

		return
	}

	names := rule.ResourceNames
	if len(names) == 0 {
		names = []string{""}
	}

	for _, group := range rule.APIGroups {
		for _, resource := range rule.Resources {
			for _, name := range names {
				for _, verb := range rule.Verbs {
					into.add(resourceTuple(group, resource, name, verb))
				}
			}
		}
	}
}

// expandRenderedRules turns the rules of a rendered role into tuples.
func expandRenderedRules(rules []rbacv1.PolicyRule) tupleSet {
	out := tupleSet{}

	for _, r := range rules {
		expandPolicyRule(rbacyaml.PolicyRule{
			APIGroups: r.APIGroups, Resources: r.Resources, ResourceNames: r.ResourceNames, NonResourceURLs: r.NonResourceURLs, Verbs: r.Verbs,
		}, out)
	}

	return out
}

// expandModelRules splits the generated rules of an object into the tuples rendered always and the
// tuples rendered only under a condition.
func expandModelRules(rules []generate.Rule) (tupleSet, tupleSet) {
	always, conditional := tupleSet{}, tupleSet{}

	for _, r := range rules {
		if r.When != "" {
			expandPolicyRule(r.PolicyRule, conditional)
		} else {
			expandPolicyRule(r.PolicyRule, always)
		}
	}

	return always, conditional
}

// lineageSet is the set of aggregation edges of a capability, as "lineage=level".
type lineageSet map[string]struct{}

func lineagesOfLabels(labels map[string]string) lineageSet {
	out := lineageSet{}

	for key, value := range labels {
		m := aggregateLabelRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}

		out[m[1]+"="+value] = struct{}{}
	}

	return out
}

func (s lineageSet) minus(other lineageSet) []string {
	var out []string

	for l := range s {
		if _, ok := other[l]; !ok {
			out = append(out, l)
		}
	}

	sort.Strings(out)

	return out
}
