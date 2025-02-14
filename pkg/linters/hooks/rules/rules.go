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
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

type HookRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

func NewHookRule(cfg *config.HooksSettings) *HookRule {
	return &HookRule{
		RuleMeta: pkg.RuleMeta{
			Name: "ingress",
		},
		BoolRule: pkg.BoolRule{
			Exclude: cfg.Ingress.Disable,
		},
	}
}

func (l *HookRule) CheckIngressCopyCustomCertificateRule(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(l.GetName()).WithFilePath(object.ShortPath())

	const (
		copyCustomCertificateImport = `"github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"`
	)

	if !l.Enabled() {
		return
	}

	if object.Unstructured.GetKind() != "Ingress" {
		return
	}

	var imports = make(map[string]struct{})
	for _, hookPath := range collectGoHooks(m.GetPath()) {
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

func collectGoHooks(moduleDir string) []string {
	goHooks := make([]string, 0)

	_ = filepath.Walk(moduleDir, func(path string, _ os.FileInfo, err error) error {
		switch {
		case err != nil:
			return err

		case strings.HasSuffix(path, "test.go"): // ignore tests
			return nil

		case strings.HasSuffix(path, ".go"):
			goHooks = append(goHooks, path)

		default:
			return nil
		}

		return nil
	})

	return goHooks
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
