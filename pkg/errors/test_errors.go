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

// TestErrorsList is a separate error list for test results (not tied to linter).
type TestErrorsList struct {
	storage *testErrStorage

	testID   string
	moduleID string
}

type testErrStorage struct {
	mu      sync.Mutex
	errList []pkg.TestError
}

func (s *testErrStorage) GetErrors() []pkg.TestError {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]pkg.TestError, 0, len(s.errList))
	result = append(result, s.errList...)

	return result
}

func (s *testErrStorage) add(err *pkg.TestError) {
	s.mu.Lock()
	s.errList = append(s.errList, *err)
	s.mu.Unlock()
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

func (l *TestErrorsList) Error(str string) *TestErrorsList {
	return l.add(str, pkg.Error)
}

func (l *TestErrorsList) Errorf(template string, a ...any) *TestErrorsList {
	return l.add(fmt.Sprintf(template, a...), pkg.Error)
}

func (l *TestErrorsList) add(str string, level pkg.Level) *TestErrorsList {
	if l.storage == nil {
		l.storage = &testErrStorage{}
	}

	e := pkg.TestError{
		TestID:   strings.ToLower(l.testID),
		ModuleID: l.moduleID,
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
