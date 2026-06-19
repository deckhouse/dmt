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

package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgerrors "github.com/deckhouse/dmt/pkg/errors"
)

const modulePath = "testdata/module"

func TestTemplatesTesterMatchesSnapshot(t *testing.T) {
	errorList := pkgerrors.NewTestErrorsList()

	tester := New(errorList, false)
	applicable := tester.Run(modulePath)

	require.True(t, applicable, "module ships template tests, tester should be applicable")
	assert.Empty(t, errorList.GetErrors(), "rendered output should match the committed snapshot")
}

func TestTemplatesTesterDetectsMismatch(t *testing.T) {
	errorList := pkgerrors.NewTestErrorsList()

	tester := New(errorList, false)
	// Point at a module without test cases to confirm the tester is skipped.
	applicable := tester.Run("testdata/does-not-exist")

	assert.False(t, applicable)
	assert.Empty(t, errorList.GetErrors())
}
