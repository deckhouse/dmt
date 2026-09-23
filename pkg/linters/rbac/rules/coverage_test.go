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

package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// writeModule lays out a module directory from a map of relative paths to contents.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()

	modulePath := filepath.Join(t.TempDir(), "module")

	for rel, content := range files {
		full := filepath.Join(modulePath, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}

	return modulePath
}

func crdYAML(group, plural, scope string) string {
	return "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: " + plural + "." + group +
		"\nspec:\n  group: " + group + "\n  names:\n    plural: " + plural + "\n    kind: X\n  scope: " + scope + "\n  versions: []\n"
}

func coverageModule(t *testing.T, path string) *mocks.ModuleMock {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(path)

	return m
}

func runCoverage(t *testing.T, modulePath string, excludes ...string) *errors.LintRuleErrorsList {
	t.Helper()

	errorList := errors.NewLintRuleErrorsList()
	rule := NewCoverageRule(pkg.StringRuleExcludeList(excludes).Get(), coverageModule(t, modulePath), errorList)
	rule.Check(context.Background())

	return errorList
}

func texts(errorList *errors.LintRuleErrorsList) []string {
	errs := errorList.GetErrors()
	out := make([]string, 0, len(errs))

	for _, e := range errs {
		out = append(out, e.Level.String()+": "+e.Text)
	}

	return out
}

func TestModuleCRDs(t *testing.T) {
	modulePath := writeModule(t, map[string]string{
		// nested directory, two documents in one file, a README and a translation to skip
		"crds/vendor/a.yaml": crdYAML("a.io", "alphas", "Namespaced") + "---\n" + crdYAML("a.io", "betas", "Cluster"),
		"crds/b.yml":         crdYAML("b.io", "gammas", "Cluster"),
		"crds/README.md":     "# not yaml\n",
		"crds/doc-ru-a.yaml": "spec: {group: a.io}\n",
		"crds/other.yaml":    "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: x}\n",
		"images/img/testdata/crds/look-alike.yaml": crdYAML("z.io", "zetas", "Cluster"),
	})

	crds, skipped := moduleCRDs(modulePath)
	require.Empty(t, skipped)

	keys := make([]string, 0, len(crds))
	for _, c := range crds {
		keys = append(keys, c.Key()+":"+c.Scope+"@"+c.File)
	}

	assert.Equal(t, []string{
		"a.io/alphas:Namespaced@crds/vendor/a.yaml",
		"a.io/betas:Cluster@crds/vendor/a.yaml",
		"b.io/gammas:Cluster@crds/b.yml",
	}, keys, "CRDs are selected by kind at any depth under crds/, and only there")
}

func TestCoverage_WithoutDeclarationIsSilent(t *testing.T) {
	modulePath := writeModule(t, map[string]string{"crds/a.yaml": crdYAML("a.io", "alphas", "Namespaced")})

	assert.Empty(t, runCoverage(t, modulePath).GetErrors(), "a module without rbac.yaml gets the contract check only (R22)")
}

func TestCoverage_FindingsAndStubFix(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	const declaration = `apiVersion: rbac.deckhouse.io/v1alpha1
# keep me: comments survive the autofix
resources:
  - group: a.io
    resource: alphas
    namespace:
      viewer: [get, list, watch]
  - group: a.io
    resource: betas
    noAccess: "TODO"
  - group: a.io
    resource: gamas   # misspelled
    scope: Cluster
    system:
      viewer: [get]
  - group: external.io
    resource: "*"
    scope: Cluster
    reason: "created at runtime"
    system:
      viewer: [get]
`

	modulePath := writeModule(t, map[string]string{
		"crds/a.yaml":     crdYAML("a.io", "alphas", "Namespaced") + "---\n" + crdYAML("a.io", "betas", "Cluster") + "---\n" + crdYAML("a.io", "gammas", "Cluster"),
		"crds/d.yaml":     crdYAML("d.io", "deltas", "Namespaced"),
		rbacyaml.Filename: declaration,
	})

	errorList := runCoverage(t, modulePath)
	got := texts(errorList)

	require.Len(t, got, 4, "got: %v", got)
	assert.Contains(t, got, "error: CRD a.io/gammas (crds/a.yaml) has no entry in rbac.yaml: decide the user access to it -- namespace, system or legacy levels, or noAccess with the reason; `dmt lint --linter rbac --fix` adds an undecided stub")
	assert.Contains(t, got, "error: CRD d.io/deltas (crds/d.yaml) has no entry in rbac.yaml: decide the user access to it -- namespace, system or legacy levels, or noAccess with the reason; `dmt lint --linter rbac --fix` adds an undecided stub")
	assert.Contains(t, got, `error: a.io/betas is still noAccess: "TODO" in rbac.yaml: a decision is needed -- grant levels, or replace "TODO" with the reason users get no access; only a person can close this`)
	assert.Contains(t, got, "warn: a.io/gamas names a resource the module's CRDs of group a.io do not have; check the spelling, or drop the entry if the resource is gone")

	// --fix: two stubs are written, and both findings stay, each with the reason (R33).
	fixes := errorList.GetFixes()
	require.Len(t, fixes, 2, "only the two missing entries carry an autofix")

	for _, fix := range fixes {
		fix()
	}

	remaining := errorList.GetErrors()
	require.Len(t, remaining, 4, "a stub is not a decision: the findings stay after --fix")

	var fixErrors int

	for _, e := range remaining {
		if e.FixError != nil {
			fixErrors++

			assert.Contains(t, e.FixError.Error(), `was added to rbac.yaml; decide its access (noAccess: "TODO" is not a decision)`)
		}
	}

	assert.Equal(t, 2, fixErrors)

	after, err := os.ReadFile(rbacyaml.Path(modulePath))
	require.NoError(t, err)
	assert.Contains(t, string(after), "# keep me: comments survive the autofix")
	assert.Contains(t, string(after), "- group: a.io\n    resource: gammas\n    noAccess: \"TODO\"\n")
	assert.Contains(t, string(after), "- group: d.io\n    resource: deltas\n    noAccess: \"TODO\"\n")

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err, "the file the autofix wrote still parses")
	assert.Len(t, decl.Resources, 6)

	// The next run: no missing entries, three undecided stubs, the misspelling still flagged.
	second := texts(runCoverage(t, modulePath))
	assert.Len(t, second, 4, "got: %v", second)
	assert.NotContains(t, strings.Join(second, "\n"), "has no entry")
	assert.Equal(t, 3, strings.Count(strings.Join(second, "\n"), `is still noAccess: "TODO"`))

	// Idempotency (R17): a run that has nothing to add carries no fixes, and appendStub itself
	// leaves a present entry alone byte for byte.
	third := runCoverage(t, modulePath)
	assert.Empty(t, third.GetFixes())

	added, err := appendStub(rbacyaml.Path(modulePath), "a.io", "alphas")
	require.NoError(t, err)
	assert.False(t, added)

	unchanged, err := os.ReadFile(rbacyaml.Path(modulePath))
	require.NoError(t, err)
	assert.Equal(t, string(after), string(unchanged))
}

// exclude-rules.coverage names a resource: neither the missing-entry finding nor the misspelling
// warning of an entry with that key is reported.
func TestCoverage_ExcludedResourceSilencesTheSpellingWarning(t *testing.T) {
	modulePath := writeModule(t, map[string]string{
		"crds/a.yaml":     crdYAML("a.io", "alphas", "Namespaced"),
		rbacyaml.Filename: "apiVersion: rbac.deckhouse.io/v1alpha1\nresources:\n  - group: a.io\n    resource: alphaz\n    scope: Namespaced\n    namespace: {viewer: [get]}\n",
	})

	got := texts(runCoverage(t, modulePath, "a.io/alphas", "a.io/alphaz"))
	assert.Empty(t, got, "got: %v", got)

	got = texts(runCoverage(t, modulePath, "a.io/alphas"))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "alphaz names a resource the module's CRDs of group a.io do not have")
}

func TestAppendStub_ScalarResources(t *testing.T) {
	modulePath := writeModule(t, map[string]string{
		rbacyaml.Filename: "apiVersion: rbac.deckhouse.io/v1alpha1\nresources: null\n",
	})

	added, err := appendStub(rbacyaml.Path(modulePath), "a.io", "alphas")
	require.NoError(t, err)
	assert.True(t, added)

	content, err := os.ReadFile(rbacyaml.Path(modulePath))
	require.NoError(t, err)
	assert.Contains(t, string(content), "resources:\n")
	assert.Contains(t, string(content), "resource: alphas")
	assert.NotContains(t, string(content), "resources: null")
}

func TestCoverage_ExcludedCRDAndDeclarationWithoutResources(t *testing.T) {
	modulePath := writeModule(t, map[string]string{
		"crds/a.yaml":     crdYAML("a.io", "alphas", "Namespaced") + "---\n" + crdYAML("a.io", "betas", "Cluster"),
		rbacyaml.Filename: "apiVersion: rbac.deckhouse.io/v1alpha1\n",
	})

	got := texts(runCoverage(t, modulePath, "a.io/betas"))
	require.Len(t, got, 1, "the excluded CRD is not required, got: %v", got)
	assert.Contains(t, got[0], "CRD a.io/alphas")

	// The autofix creates the resources list in a declaration that has none yet.
	errorList := runCoverage(t, modulePath, "a.io/betas")
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)
	require.Len(t, decl.Resources, 1)
	assert.Equal(t, "a.io/alphas", decl.Resources[0].Key())
	assert.Equal(t, rbacyaml.NoAccessTODO, decl.Resources[0].NoAccess)
}

// R36: two render variants report the same missing entry; the stub is written once and both
// findings end the run with the same outcome.
func TestCoverage_StubFixOncePerRun(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := writeModule(t, map[string]string{
		"crds/a.yaml":     crdYAML("a.io", "alphas", "Namespaced"),
		rbacyaml.Filename: "apiVersion: rbac.deckhouse.io/v1alpha1\nresources: []\n",
	})

	variantA := runCoverage(t, modulePath)
	variantB := runCoverage(t, modulePath)

	for _, list := range []*errors.LintRuleErrorsList{variantA, variantB} {
		for _, fix := range list.GetFixes() {
			fix()
		}
	}

	for _, list := range []*errors.LintRuleErrorsList{variantA, variantB} {
		remaining := list.GetErrors()
		require.Len(t, remaining, 1)
		require.Error(t, remaining[0].FixError)
		assert.Contains(t, remaining[0].FixError.Error(), "a stub for a.io/alphas was added to rbac.yaml")
	}

	after, err := os.ReadFile(rbacyaml.Path(modulePath))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(after), "resource: alphas"), "one stub, not one per variant")
}

// A noAccess entry whose group has no CRD in the module and no scope: a removed CRD is
// indistinguishable from an external resource, so the entry is asked to say which.
func TestCoverage_DeniedEntryWithoutCRDNeedsScope(t *testing.T) {
	modulePath := writeModule(t, map[string]string{
		"crds/a.yaml": crdYAML("a.io", "alphas", "Namespaced"),
		rbacyaml.Filename: `apiVersion: rbac.deckhouse.io/v1alpha1
resources:
  - group: a.io
    resource: alphas
    noAccess: "internal"
  - group: gone.io
    resource: relics
    noAccess: "the CRD left with the old controller"
  - group: external.io
    resource: things
    scope: Cluster
    noAccess: "documented denial of an external resource"
`,
	})

	got := texts(runCoverage(t, modulePath))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "warn: gone.io/relics is denied access but the module ships no CRD for it and the entry names no scope")
}

// One CRD document that does not parse is skipped with a warning; the other CRDs are still
// covered (review of #479, finding 13i).
func TestCoverage_BadCRDDocumentIsSkipped(t *testing.T) {
	modulePath := writeModule(t, map[string]string{
		"crds/a.yaml":     crdYAML("a.io", "alphas", "Namespaced") + "---\n{{ if .Values.x }}: [\n",
		rbacyaml.Filename: "apiVersion: rbac.deckhouse.io/v1alpha1\n",
	})

	got := texts(runCoverage(t, modulePath))
	joined := strings.Join(got, "\n")
	assert.Contains(t, joined, "warn: a CRD document is skipped: parse crds/a.yaml")
	assert.Contains(t, joined, "a.io/alphas", "the CRD that parses is still judged")
}
