package ingress

import (
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

func ingressCopyCustomCertificateRule(m *module.Module, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	const (
		copyCustomCertificateImport = `"github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"`
	)

	if slices.Contains(Cfg.SkipIngressChecks, m.GetName()) {
		return nil
	}

	if object.Unstructured.GetKind() != "Ingress" {
		return nil
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
		return result.WithObjectID(m.GetName()).Add(
			"Ingress does not contain copy_custom_certificate hook",
		)
	}

	return nil
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
