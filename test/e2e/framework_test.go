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

package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
)

func TestMatchExpectPass(t *testing.T) {
	t.Parallel()

	findings := []pkg.LinterError{
		{LinterID: "container", RuleID: "object-recommended-labels", Level: pkg.Error, Text: `missing "module"`},
	}

	spec := &CaseSpec{
		Expect: []Finding{
			Finding{Linter: "container", Rule: "object-recommended-labels"},
		},
		ExpectPass: []Finding{
			{Linter: "container", Rule: "object-namespace-labels"},
		},
	}

	require.True(t, Match(spec, findings).OK())
}

func TestMatchExpectPassFailsWhenFindingPresent(t *testing.T) {
	t.Parallel()

	findings := []pkg.LinterError{
		{LinterID: "container", RuleID: "object-namespace-labels", Level: pkg.Error, Text: "missing label"},
	}

	spec := &CaseSpec{
		ExpectPass: []Finding{
			{Linter: "container", Rule: "object-namespace-labels"},
		},
	}

	result := Match(spec, findings)
	require.False(t, result.OK())
	require.Contains(t, result.Failures[0], "expected rule to pass")
}

func TestMatchExpectPassFiltersByLevelAndText(t *testing.T) {
	t.Parallel()

	findings := []pkg.LinterError{
		{LinterID: "container", RuleID: "object-namespace-labels", Level: pkg.Ignored, Text: "ignored issue"},
	}

	spec := &CaseSpec{
		ExpectPass: []Finding{
			{Linter: "container", Rule: "object-namespace-labels", Level: "error"},
		},
	}

	require.True(t, Match(spec, findings).OK())
}
