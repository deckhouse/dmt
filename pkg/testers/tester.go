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

package testers

import (
	"errors"
	"fmt"
)

var ErrNotApplicable = errors.New("not applicable")

type notApplicableError struct {
	reason string
}

func (e *notApplicableError) Error() string {
	return fmt.Sprintf("not applicable: %s", e.reason)
}

func (e *notApplicableError) Is(target error) bool {
	return target == ErrNotApplicable
}

func NotApplicable(reason string) error {
	return &notApplicableError{reason: reason}
}

type TestError struct {
	TestName string
	Message  string
	Expected string
	Got      string
}

func (e *TestError) Error() string {
	if e.Expected != "" && e.Got != "" {
		return fmt.Sprintf("%s: %s; expected: %s; got: %s", e.TestName, e.Message, e.Expected, e.Got)
	}
	return fmt.Sprintf("%s: %s", e.TestName, e.Message)
}

// Tester is the interface that all module testers must implement.
// Each tester validates a specific aspect of a module (conversions, values, render, etc.)
type Tester interface {
	// Run executes the test on the given module path.
	// Returns nil if test passed, returns error describing the failure.
	// Returns ErrNotApplicable if the test is not applicable to this module.
	Run(modulePath string) error

	// Name returns the test name (e.g., "conversions", "values").
	Name() string

	// Desc returns a short description of what the test does.
	Desc() string
}
