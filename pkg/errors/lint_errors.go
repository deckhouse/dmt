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

package errors

import (
	"fmt"
	"strings"
	"sync"

	"github.com/deckhouse/dmt/pkg"
)

// AutofixFunc applies an automatic fix for a single finding.
// It is expected to be idempotent: applying it to an already-fixed source must
// be a no-op and must not error.
type AutofixFunc func() error

type lintRuleError struct {
	LinterID    string
	ModuleID    string
	RuleID      string
	ObjectID    string
	ObjectValue any
	Text        string
	FilePath    string
	LineNumber  int
	Level       pkg.Level

	// fix, when set, knows how to automatically resolve this finding.
	// It is applied by LintRuleErrorsList.ApplyFixes when dmt runs with --fix.
	fix AutofixFunc
}

func (l *lintRuleError) EqualsTo(candidate lintRuleError) bool { //nolint:gocritic // it's a simple method
	return l.LinterID == candidate.LinterID &&
		l.Text == candidate.Text &&
		l.ObjectID == candidate.ObjectID &&
		l.ModuleID == candidate.ModuleID
}

type errStorage struct {
	mu      sync.Mutex
	errList []lintRuleError
}

func (s *errStorage) GetErrors() []lintRuleError {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]lintRuleError, 0, len(s.errList))
	result = append(result, s.errList...)

	return result
}

func (s *errStorage) add(err *lintRuleError) {
	s.mu.Lock()
	s.errList = append(s.errList, *err)
	s.mu.Unlock()
}

// FixResult summarizes the outcome of ApplyFixes.
type FixResult struct {
	Applied int     // number of findings resolved automatically
	Failed  []error // errors returned by fixes that could not be applied
}

type LintRuleErrorsList struct {
	storage *errStorage

	linterID   string
	moduleID   string
	ruleID     string
	objectID   string
	value      any
	filePath   string
	lineNumber int
	fix        AutofixFunc

	maxLevel *pkg.Level
}

func NewLintRuleErrorsList() *LintRuleErrorsList {
	lvl := pkg.Error

	return &LintRuleErrorsList{
		storage: &errStorage{
			errList: make([]lintRuleError, 0),
		},
		maxLevel: &lvl,
	}
}

func (l *LintRuleErrorsList) copy() *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	t := *l

	return &t
}

func (l *LintRuleErrorsList) WithMaxLevel(level *pkg.Level) *LintRuleErrorsList {
	list := l.copy()
	list.maxLevel = level

	return list
}

func (l *LintRuleErrorsList) WithLinterID(linterID string) *LintRuleErrorsList {
	list := l.copy()
	list.linterID = linterID

	return list
}

func (l *LintRuleErrorsList) WithModule(moduleID string) *LintRuleErrorsList {
	list := l.copy()
	list.moduleID = moduleID

	return list
}

func (l *LintRuleErrorsList) WithRule(ruleID string) *LintRuleErrorsList {
	list := l.copy()
	list.ruleID = ruleID

	return list
}

func (l *LintRuleErrorsList) WithObjectID(objectID string) *LintRuleErrorsList {
	list := l.copy()
	list.objectID = objectID

	return list
}

func (l *LintRuleErrorsList) WithValue(value any) *LintRuleErrorsList {
	list := l.copy()
	list.value = value

	return list
}

func (l *LintRuleErrorsList) WithFilePath(filePath string) *LintRuleErrorsList {
	list := l.copy()
	list.filePath = filePath

	return list
}

func (l *LintRuleErrorsList) WithLineNumber(lineNumber int) *LintRuleErrorsList {
	list := l.copy()
	list.lineNumber = lineNumber

	return list
}

// WithFix attaches an automatic fix to the next emitted finding.
// Findings that carry a fix are resolved by ApplyFixes when dmt runs with --fix.
func (l *LintRuleErrorsList) WithFix(fix AutofixFunc) *LintRuleErrorsList {
	list := l.copy()
	list.fix = fix

	return list
}

func (l *LintRuleErrorsList) Warn(str string) *LintRuleErrorsList {
	return l.add(str, pkg.Warn)
}

func (l *LintRuleErrorsList) Warnf(template string, a ...any) *LintRuleErrorsList {
	return l.add(fmt.Sprintf(template, a...), pkg.Warn)
}

func (l *LintRuleErrorsList) Error(str string) *LintRuleErrorsList {
	return l.add(str, pkg.Error)
}

func (l *LintRuleErrorsList) Errorf(template string, a ...any) *LintRuleErrorsList {
	return l.add(fmt.Sprintf(template, a...), pkg.Error)
}

func (l *LintRuleErrorsList) add(str string, level pkg.Level) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	if l.maxLevel != nil && *l.maxLevel < level {
		level = *l.maxLevel
	}

	e := lintRuleError{
		LinterID:    strings.ToLower(l.linterID),
		ModuleID:    l.moduleID,
		RuleID:      l.ruleID,
		ObjectID:    l.objectID,
		ObjectValue: l.value,
		FilePath:    l.filePath,
		LineNumber:  l.lineNumber,
		Text:        str,
		Level:       level,
		fix:         l.fix,
	}

	l.storage.add(&e)

	return l
}

func (l *LintRuleErrorsList) GetErrors() []pkg.LinterError {
	return remapErrorsToLinterErrors(l.storage.GetErrors()...)
}

// ApplyFixes runs every fix attached to a collected finding and drops the
// findings that were resolved successfully from the list. Findings without a
// fix, or whose fix failed, are kept so they are still reported.
//
// This is the single entry point used by --fix: each rule attaches a fix to the
// findings it produces, and ApplyFixes processes all of them in one place.
func (l *LintRuleErrorsList) ApplyFixes() FixResult {
	if l.storage == nil {
		return FixResult{}
	}

	l.storage.mu.Lock()
	defer l.storage.mu.Unlock()

	var result FixResult

	remaining := make([]lintRuleError, 0, len(l.storage.errList))

	for idx := range l.storage.errList {
		e := l.storage.errList[idx]

		if e.fix == nil {
			remaining = append(remaining, e)
			continue
		}

		if err := e.fix(); err != nil {
			result.Failed = append(result.Failed, fmt.Errorf("%s: %w", e.Text, err))
			remaining = append(remaining, e)

			continue
		}

		result.Applied++
	}

	l.storage.errList = remaining

	return result
}

func (l *LintRuleErrorsList) ContainsErrors() bool {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	errs := l.storage.GetErrors()

	for idx := range errs {
		err := errs[idx]

		if err.Level == pkg.Error {
			return true
		}
	}

	return false
}

func remapErrorsToLinterErrors(errs ...lintRuleError) []pkg.LinterError {
	result := make([]pkg.LinterError, 0, len(errs))

	for idx := range errs {
		result = append(result, *remapErrorToLinterError(&errs[idx]))
	}

	return result
}

func remapErrorToLinterError(err *lintRuleError) *pkg.LinterError {
	return &pkg.LinterError{
		LinterID:    err.LinterID,
		ModuleID:    err.ModuleID,
		RuleID:      err.RuleID,
		ObjectID:    err.ObjectID,
		ObjectValue: err.ObjectValue,
		FilePath:    err.FilePath,
		LineNumber:  err.LineNumber,
		Text:        err.Text,
		Level:       err.Level,
	}
}
