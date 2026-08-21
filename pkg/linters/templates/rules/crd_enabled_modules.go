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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	CRDEnabledModulesRuleName = "crd-enabled-modules"

	// crdModuleSuffix is the suffix of the deprecated standalone CRD modules
	// (e.g. "operator-prometheus-crd"). Their CRDs were merged into the parent
	// module, so the "-crd" name must be dropped in enabledModules checks.
	crdModuleSuffix = "-crd"

	// moduleConfigFilename is the module definition file that declares, among
	// other things, requirements.deckhouse — the module's target platform version.
	moduleConfigFilename = "module.yaml"

	// MinimalDeckhouseVersionForCRDModulesRemoval is the Deckhouse version starting
	// from which the standalone "<module>-crd" modules were removed (Deckhouse PR
	// #9593 "Get rid of crd modules", first released in v1.65.0). Their CRDs are now
	// discovered and installed automatically from the parent module's crds/ directory,
	// so a separate "-crd" module no longer exists and referencing its name through
	// .Values.global.enabledModules is a deprecated pattern that should use the parent
	// module name (without the "-crd" suffix) instead.
	//
	// The rule only fires for modules whose requirements.deckhouse constraint starts
	// at or above this version; modules that still support older Deckhouse releases
	// (or declare no deckhouse requirement) are out of scope, mirroring the other
	// version-gated checks in dmt.
	MinimalDeckhouseVersionForCRDModulesRemoval = "1.65.0"
)

// crdEnabledModulesRe captures a module name referenced through
// `.Values.global.enabledModules | has "<name>"`. Capture group 1 is the module
// name, which the rule then tests for the "-crd" suffix.
var crdEnabledModulesRe = regexp.MustCompile(`\.Values\.global\.enabledModules\s*\|\s*has\s+"([^"]*)"`)

// crdVersionConstraintRegex extracts operator+version pairs (e.g. ">= 1.77") from a
// semver constraint string, mirroring the module linter's requirements handling.
var crdVersionConstraintRegex = regexp.MustCompile(`([><=]=?|!=)\s*v?(\d+(?:\.\d+){0,2})`)

type CRDEnabledModulesRule struct {
	pkg.RuleMeta
}

func NewCRDEnabledModulesRule() *CRDEnabledModulesRule {
	return &CRDEnabledModulesRule{
		RuleMeta: pkg.RuleMeta{
			Name: CRDEnabledModulesRuleName,
		},
	}
}

// CheckCRDEnabledModules scans the module templates for deprecated references to
// standalone "-crd" modules through .Values.global.enabledModules and reports each
// one, offering an autofix that drops the "-crd" suffix. The check is gated on the
// module's target Deckhouse version (see MinimalDeckhouseVersionForCRDModulesRemoval).
func (r *CRDEnabledModulesRule) CheckCRDEnabledModules(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	// The standalone "-crd" modules were removed starting from a specific Deckhouse
	// version. Only modules that target that version (or newer) must stop referencing
	// the "-crd" names; skip everything else to avoid false positives on modules that
	// still support older releases where the "-crd" module still exists.
	if !moduleTargetsDeckhouseAtLeast(m.GetPath(), MinimalDeckhouseVersionForCRDModulesRemoval) {
		return
	}

	templatesPath := filepath.Join(m.GetPath(), "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	files := fsutils.GetFiles(templatesPath, true, fsutils.FilterFileByExtensions(".yaml", ".yml", ".tpl"))

	for _, filePath := range files {
		relPath := fsutils.Rel(m.GetPath(), filePath)

		content, err := os.ReadFile(filePath)
		if err != nil {
			errorList.WithFilePath(relPath).Errorf("Failed to read file: %v", err)
			continue
		}

		for _, match := range crdEnabledModulesRe.FindAllSubmatchIndex(content, -1) {
			name := string(content[match[2]:match[3]])
			if !strings.HasSuffix(name, crdModuleSuffix) {
				continue
			}

			suggested := strings.TrimSuffix(name, crdModuleSuffix)
			line := bytes.Count(content[:match[0]], []byte("\n")) + 1

			errorList.
				WithFilePath(relPath).
				WithLineNumber(line).
				WithValue(name).
				WithFix(fixCRDEnabledModuleReferences(filePath)).
				Errorf(
					"Deprecated %q reference in .Values.global.enabledModules: standalone %q modules "+
						"were removed in Deckhouse v%s, use %q instead.",
					name, crdModuleSuffix, MinimalDeckhouseVersionForCRDModulesRemoval, suggested,
				)
		}
	}
}

// fixCRDEnabledModuleReferences returns an idempotent autofix that rewrites every
// `.Values.global.enabledModules | has "<name>-crd"` occurrence in filePath to use
// the parent module name (without the "-crd" suffix). Only the module-name spans are
// rewritten, so everything else in the file is preserved byte-for-byte. Because the
// fix rewrites all "-crd" references in the file, running it once resolves every
// finding for that file and a second run is a no-op — safe when several findings in
// the same file each carry this fix.
func fixCRDEnabledModuleReferences(filePath string) errors.AutofixFunc {
	return func() error {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(filePath), err)
		}

		var out bytes.Buffer

		last := 0
		changed := false

		for _, match := range crdEnabledModulesRe.FindAllSubmatchIndex(content, -1) {
			nameStart, nameEnd := match[2], match[3]

			name := string(content[nameStart:nameEnd])
			if !strings.HasSuffix(name, crdModuleSuffix) {
				continue
			}

			out.Write(content[last:nameStart])
			out.WriteString(strings.TrimSuffix(name, crdModuleSuffix))

			last = nameEnd
			changed = true
		}

		if !changed {
			return nil
		}

		out.Write(content[last:])

		return writeFileAtomically(filePath, out.Bytes())
	}
}

// writeFileAtomically writes data to path via a temp file + rename, preserving the
// original file mode. The temp file is removed if the rename fails.
func writeFileAtomically(path string, data []byte) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", filepath.Base(path), err)
	}

	tmpFile := path + ".fix.tmp"
	if err := os.WriteFile(tmpFile, data, fi.Mode()); err != nil {
		return fmt.Errorf("write temp file for %s: %w", filepath.Base(path), err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename temp file for %s: %w", filepath.Base(path), err)
	}

	return nil
}

// moduleTargetsDeckhouseAtLeast reports whether the module.yaml at modulePath declares
// a requirements.deckhouse constraint whose minimal allowed version is >= minVersion.
// Modules without a deckhouse requirement (or with an unparseable constraint) are
// treated as out of scope and return false.
func moduleTargetsDeckhouseAtLeast(modulePath, minVersion string) bool {
	constraintStr := readDeckhouseRequirement(modulePath)
	if constraintStr == "" {
		return false
	}

	constraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return false
	}

	minAllowed := minimalAllowedVersion(constraint)
	if minAllowed == nil {
		return false
	}

	minVer, err := semver.NewVersion(minVersion)
	if err != nil {
		return false
	}

	return !minAllowed.LessThan(minVer)
}

// crdModuleMeta is the minimal slice of module.yaml this rule needs: the target
// Deckhouse version constraint. module.yaml is parsed with sigs.k8s.io/yaml (YAML via
// JSON), matching how the module linter reads it, so the tags are json tags.
type crdModuleMeta struct {
	Requirements struct {
		Deckhouse string `json:"deckhouse"`
	} `json:"requirements"`
}

// readDeckhouseRequirement returns the raw requirements.deckhouse constraint string
// from the module.yaml at modulePath, or "" when the file is missing/unreadable or the
// field is absent.
func readDeckhouseRequirement(modulePath string) string {
	data, err := os.ReadFile(filepath.Join(modulePath, moduleConfigFilename))
	if err != nil {
		return ""
	}

	var meta crdModuleMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return ""
	}

	return meta.Requirements.Deckhouse
}

// minimalAllowedVersion finds the lowest version bound among the >=, >, and =
// operators in constraint. != is deliberately ignored — it does not set a lower
// bound. Returns nil when only <, <=, or != are present. This mirrors
// findMinimalAllowedVersion in the module linter.
func minimalAllowedVersion(constraint *semver.Constraints) *semver.Version {
	if constraint == nil {
		return nil
	}

	var minVersion *semver.Version

	for _, m := range crdVersionConstraintRegex.FindAllStringSubmatch(constraint.String(), -1) {
		if len(m) < 3 {
			continue
		}

		op := m[1]
		if op != ">=" && op != ">" && op != "=" {
			continue
		}

		v, err := semver.NewVersion(m[2])
		if err != nil {
			continue
		}

		if minVersion == nil || v.LessThan(minVersion) {
			minVersion = v
		}
	}

	return minVersion
}
