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
	ignoreUnknownFields: true,
}

// CheckRules validates rule files.
func CheckRules(data []byte) error {
	rgs, errs := rulefmt.Parse(data, ls.ignoreUnknownFields)
	var ruleErrors, checkGroupErrors error
	if errs != nil {
		errMessage := "  FAILED:"
		for _, e := range errs {
			errMessage += fmt.Sprintf("\n%s", e.Error())
		}
		ruleErrors = errors.New(errMessage)
	}
	if _, errs := checkRuleGroups(rgs, ls); errs != nil {
		errMessage := "  FAILED:"
		for _, e := range errs {
			errMessage += fmt.Sprintf("\n%s", e.Error())
		}
		checkGroupErrors = errors.New(errMessage)
	}

	return errors.Join(ruleErrors, checkGroupErrors)
}

func checkRuleGroups(rgs *rulefmt.RuleGroups, lintSettings rulesLintConfig) (int, []error) {
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
			return 0, []error{fmt.Errorf("%w %s", errLint, errMessage)}
		}
	}

	return numRules, nil
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
	var cRules compareRuleTypes

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

func ruleMetric(rule rulefmt.Rule) string {
	if rule.Alert != "" {
		return rule.Alert
	}
	return rule.Record
}
