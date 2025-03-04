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

type enabledFn func() bool
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

type LintRuleErrorsList struct {
	storage *errStorage

	linterID   string
	moduleID   string
	ruleID     string
	objectID   string
	value      any
	filePath   string
	lineNumber int

	maxLevel *pkg.Level

	enabled enabledFn
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

// NewLinterRuleList creates a new LintRuleErrorsList with a specified linterID and moduleID.
// If moduleID is not provided, it will be empty.
// Used only first moduleID for a linter.
func NewLinterRuleList(linterID string, module ...string) *LintRuleErrorsList {
	l := &LintRuleErrorsList{
		storage:  &errStorage{},
		linterID: linterID,
	}
	if len(module) > 0 {
		l.moduleID = module[0]
	}

	return l
}

func (l *LintRuleErrorsList) copy() *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	return &LintRuleErrorsList{
		storage:    l.storage,
		linterID:   l.linterID,
		moduleID:   l.moduleID,
		ruleID:     l.ruleID,
		objectID:   l.objectID,
		value:      l.value,
		filePath:   l.filePath,
		lineNumber: l.lineNumber,
		maxLevel:   l.maxLevel,
		enabled:    l.enabled,
	}
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

func (l *LintRuleErrorsList) WithEnabled(f enabledFn) *LintRuleErrorsList {
	list := l.copy()
	list.enabled = f
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

	if l.enabled != nil && !l.enabled() {
		return l
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
	}

	l.storage.add(&e)

	return l
}

func (l *LintRuleErrorsList) GetErrors() []pkg.LinterError {
	return remapErrorsToLinterErrors(l.storage.GetErrors()...)
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
