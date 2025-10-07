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

package rules

import (
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

type HookRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

func NewHookRule(cfg *pkg.HooksLinterConfig) *HookRule {
	return &HookRule{
		RuleMeta: pkg.RuleMeta{
			Name: "ingress",
		},
		BoolRule: pkg.BoolRule{
			Exclude: cfg.IngressRuleSettings.Disable,
		},
	}
}

func (l *HookRule) CheckIngressCopyCustomCertificateRule(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(l.GetName()).WithFilePath(object.GetPath())

	const (
		copyCustomCertificateImport = `"github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"`
	)

	if !l.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	if object.Unstructured.GetKind() != "Ingress" {
		return
	}

	hooksDir := filepath.Join(m.GetPath(), "hooks")
	files := fsutils.GetFiles(hooksDir, false, filterCopyCustomCertificateHook)
	if len(files) > 0 {
		return
	}

	var imports = make(map[string]struct{})
	for _, hookPath := range fsutils.GetFiles(hooksDir, false, filterGoHooks) {
		p, err := getImports(hookPath)
		if err != nil {
			continue
		}

		maps.Copy(imports, p)
	}

	if _, ok := imports[copyCustomCertificateImport]; !ok {
		errorList.Error("Ingress resource exists but module does not have copy_custom_certificate hook")
	}
}

func filterCopyCustomCertificateHook(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)
	filename := filepath.Base(path)

	if filename == "copy_custom_certificate.go" ||
		filename == "copy_custom_certificate.py" {
		return true
	}

	return false
}

func filterGoHooks(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)
	filename := filepath.Base(path)

	if strings.HasSuffix(filename, "test.go") {
		return false
	}
	if strings.HasSuffix(filename, ".go") {
		return true
	}

	return false
}

func getImports(filename string) (map[string]struct{}, error) {
	fSet := token.NewFileSet()

	astFile, err := parser.ParseFile(fSet, filename, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	var imports = make(map[string]struct{})

	for _, s := range astFile.Imports {
		imports[s.Path.Value] = struct{}{}
	}

	return imports, nil
}
