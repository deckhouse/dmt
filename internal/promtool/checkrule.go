/*
Copyright 2025 Flant JSC

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

package promtool

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/rules"

	"github.com/deckhouse/dmt/internal/promtool/rulefmt"
)

type rulesLintConfig struct {
	all                 bool
	duplicateRules      bool
	fatal               bool
	ignoreUnknownFields bool
}

var errLint = errors.New("lint error")

func (ls rulesLintConfig) lintDuplicateRules() bool {
	return ls.all || ls.duplicateRules
}

var ls = rulesLintConfig{
	all:                 true,
	duplicateRules:      true,
	fatal:               false,
	ignoreUnknownFields: false,
}

// CheckRules validates rule files.
func CheckRules(data []byte) error {
	rgs, errs := rulefmt.Parse(data, ls.ignoreUnknownFields)
	var ruleErrors, checkGroupErrors error
	if errs != nil {
		errStr := make([]string, 0, len(errs))
		for _, e := range errs {
			errStr = append(errStr, e.Error())
		}
		ruleErrors = fmt.Errorf("%s", strings.Join(errStr, "\n"))
	}
	if errs := checkRuleGroups(rgs, ls); errs != nil {
		errStr := make([]string, 0, len(errs))
		for _, e := range errs {
			errStr = append(errStr, e.Error())
		}
		checkGroupErrors = fmt.Errorf("%s", strings.Join(errStr, "\n"))
	}

	return errors.Join(ruleErrors, checkGroupErrors)
}

func checkRuleGroups(rgs *rulefmt.RuleGroups, lintSettings rulesLintConfig) []error {
	if rgs == nil || len(rgs.Groups) == 0 {
		return []error{fmt.Errorf("%w: no rule groups found", errLint)}
	}
	numRules := 0
	for _, rg := range rgs.Groups {
		numRules += len(rg.Rules)
	}

	if lintSettings.lintDuplicateRules() {
		dRules := checkDuplicates(rgs.Groups)
		if len(dRules) != 0 {
			errMessage := fmt.Sprintf("%d duplicate rule(s) found.\n", len(dRules))
			for _, n := range dRules {
				errMessage += fmt.Sprintf("Metric: %s\nLabel(s):\n", n.metric)
				n.label.Range(func(l labels.Label) {
					errMessage += fmt.Sprintf("\t%s: %s\n", l.Name, l.Value)
				})
			}
			errMessage += "Might cause inconsistency while recording expressions"
			return []error{fmt.Errorf("%w %s", errLint, errMessage)}
		}
	}

	return nil
}

type compareRuleType struct {
	metric string
	label  labels.Labels
}

type compareRuleTypes []compareRuleType

func (c compareRuleTypes) Len() int           { return len(c) }
func (c compareRuleTypes) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c compareRuleTypes) Less(i, j int) bool { return compare(c[i], c[j]) < 0 }

func compare(a, b compareRuleType) int {
	if res := strings.Compare(a.metric, b.metric); res != 0 {
		return res
	}

	return labels.Compare(a.label, b.label)
}

func checkDuplicates(groups []rulefmt.RuleGroup) []compareRuleType {
	var duplicates []compareRuleType
	cRules := make(compareRuleTypes, 0, 100) // Preallocate with reasonable capacity

	for _, group := range groups {
		for _, rule := range group.Rules {
			cRules = append(cRules, compareRuleType{
				metric: ruleMetric(rule),
				label:  rules.FromMaps(group.Labels, rule.Labels),
			})
		}
	}
	if len(cRules) < 2 {
		return duplicates
	}
	sort.Sort(cRules)

	last := cRules[0]
	for i := 1; i < len(cRules); i++ {
		if compare(last, cRules[i]) == 0 {
			// Don't add a duplicated rule multiple times.
			if len(duplicates) == 0 || compare(last, duplicates[len(duplicates)-1]) != 0 {
				duplicates = append(duplicates, cRules[i])
			}
		}
		last = cRules[i]
	}

	return duplicates
}

//nolint:gocritic // ...
func ruleMetric(rule rulefmt.Rule) string {
	if rule.Alert != "" {
		return rule.Alert
	}
	return rule.Record
}
