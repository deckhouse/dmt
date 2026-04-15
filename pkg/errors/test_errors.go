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

	"github.com/deckhouse/dmt/pkg"
)

// TestErrorsList is a separate error list for test results (not tied to linter).
type TestErrorsList struct {
	storage *testErrStorage

	testID   string
	moduleID string
	testName string
}

func NewTestErrorsList() *TestErrorsList {
	return &TestErrorsList{
		storage: &testErrStorage{
			errList: make([]pkg.TestError, 0),
		},
	}
}

func (l *TestErrorsList) copy() *TestErrorsList {
	if l.storage == nil {
		l.storage = &testErrStorage{}
	}

	t := *l

	return &t
}

func (l *TestErrorsList) WithTestID(testID string) *TestErrorsList {
	list := l.copy()
	list.testID = testID

	return list
}

func (l *TestErrorsList) WithModule(moduleID string) *TestErrorsList {
	list := l.copy()
	list.moduleID = moduleID

	return list
}

func (l *TestErrorsList) WithTestName(testName string) *TestErrorsList {
	list := l.copy()
	list.testName = testName

	return list
}

func (l *TestErrorsList) Error(str string) *TestErrorsList {
	return l.add(str, pkg.Error)
}

func (l *TestErrorsList) Errorf(template string, a ...any) *TestErrorsList {
	return l.add(fmt.Sprintf(template, a...), pkg.Error)
}

// AddTestResult adds a structured test failure with got/expected comparison data.
func (l *TestErrorsList) AddTestResult(text, testName, got, expected string) *TestErrorsList {
	if l.storage == nil {
		l.storage = &testErrStorage{}
	}

	e := pkg.TestError{
		TestID:   strings.ToLower(l.testID),
		ModuleID: l.moduleID,
		Text:     text,
		Level:    pkg.Error,
		TestName: testName,
		Got:      got,
		Expected: expected,
	}

	l.storage.add(&e)

	return l
}

func (l *TestErrorsList) add(str string, level pkg.Level) *TestErrorsList {
	if l.storage == nil {
		l.storage = &testErrStorage{}
	}

	e := pkg.TestError{
		TestID:   strings.ToLower(l.testID),
		ModuleID: l.moduleID,
		TestName: l.testName,
		Text:     str,
		Level:    level,
	}

	l.storage.add(&e)

	return l
}

func (l *TestErrorsList) GetErrors() []pkg.TestError {
	return l.storage.GetErrors()
}

func (l *TestErrorsList) ContainsErrors() bool {
	if l.storage == nil {
		l.storage = &testErrStorage{}
	}

	errs := l.storage.GetErrors()

	for idx := range errs {
		if errs[idx].Level == pkg.Error {
			return true
		}
	}

	return false
}
