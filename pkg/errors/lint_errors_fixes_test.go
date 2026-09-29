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

package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/pkg"
)

// A rule switched off with impact: ignore keeps its autofix off with it: --fix must not rewrite
// files on behalf of findings nobody sees.
func TestGetFixes_SkipsIgnoredFindings(t *testing.T) {
	list := NewLintRuleErrorsList()
	ran := map[string]int{}

	ignored := pkg.Ignored
	list.WithMaxLevel(&ignored).WithFix(func() error { ran["ignored"]++; return nil }).Errorf("switched off")
	list.WithFix(func() error { ran["active"]++; return nil }).Errorf("active")

	fixes := list.GetFixes()
	require.Len(t, fixes, 1)

	for _, fix := range fixes {
		fix()
	}

	require.Equal(t, map[string]int{"active": 1}, ran)
}

// A fix that runs and does not close its finding fails the run at any level but ignored (review
// of #479, reply to finding 9).
func TestContainsFailedFixes(t *testing.T) {
	list := NewLintRuleErrorsList().WithMaxLevel(ptr.To(pkg.Warn))
	list.WithFix(func() error { return nil }).Warn("closed by its fix")
	assert.False(t, list.ContainsFailedFixes(), "nothing ran yet")

	for _, fix := range list.GetFixes() {
		fix()
	}

	assert.False(t, list.ContainsFailedFixes(), "the fix closed its finding")

	failing := NewLintRuleErrorsList().WithMaxLevel(ptr.To(pkg.Warn))
	failing.WithFix(func() error { return stderrors.New("a decision is open") }).Warn("left open")

	for _, fix := range failing.GetFixes() {
		fix()
	}

	assert.True(t, failing.ContainsFailedFixes(), "a warn-level finding with a failed fix fails the run")
	assert.False(t, failing.ContainsErrors(), "its level stays warn")
}
