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
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"

	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	TLSCertificateRuleName = "tls-certificate"

	// tlsCertificateImport is the go_lib package whose helpers
	// (RegisterInternalTLSHook / GenerateSelfSignedCert) are known to
	// produce invalid self-signed certificates when misused.
	// See https://github.com/deckhouse/deckhouse/pull/20138.
	tlsCertificateImport = `"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"`

	// invalidUsage is a bogus usage string that does not exist in cfssl's
	// KeyUsage/ExtKeyUsage maps. cfssl silently discards it, emitting a
	// certificate with no ExtendedKeyUsage extension which strict validators
	// (Java keystores, Trivy, MaxPatrol) reject.
	invalidUsage = "requestheader-client"

	// recommendedUsage is the EKU that must be present for server certificates.
	recommendedUsage = "server auth"

	// generateSelfSignedCertFunc is the helper that issues a leaf certificate.
	generateSelfSignedCertFunc = "GenerateSelfSignedCert"

	// withGroupsOption copies the CA's O= onto the leaf, recreating the
	// Subject == Issuer (depth-0 self-signed) collision rejected by OpenSSL.
	withGroupsOption = "WithGroups"
)

type TLSCertificateRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

func NewTLSCertificateRule(disable bool) *TLSCertificateRule {
	return &TLSCertificateRule{
		RuleMeta: pkg.RuleMeta{
			Name: TLSCertificateRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
	}
}

// CheckTLSCertificateHooks scans the module's Go hooks for the self-signed
// certificate defects fixed in deckhouse/deckhouse#20138:
//
//  1. Bogus "requestheader-client" usage that results in an empty
//     ExtendedKeyUsage extension instead of "server auth".
//  2. WithGroups applied to a leaf certificate, which copies the CA's
//     Organization onto the leaf and recreates the Subject == Issuer
//     (depth-0 self-signed) collision.
func (r *TLSCertificateRule) CheckTLSCertificateHooks(m *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	modulePath := m.GetPath()
	hooksDir := filepath.Join(modulePath, "hooks")

	for _, hookPath := range fsutils.GetFiles(hooksDir, false, filterGoHooks) {
		r.checkFile(modulePath, hookPath, errorList)
	}
}

func (r *TLSCertificateRule) checkFile(modulePath, hookPath string, errorList *errors.LintRuleErrorsList) {
	fSet := token.NewFileSet()

	astFile, err := parser.ParseFile(fSet, hookPath, nil, parser.AllErrors)
	if err != nil {
		// Parsing errors are reported by the compiler/other tooling, skip here.
		return
	}

	if !fileImportsTLSCertificate(astFile) {
		return
	}

	relPath := fsutils.Rel(modulePath, hookPath)
	fileErrorList := errorList.WithFilePath(relPath)

	ast.Inspect(astFile, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			if node.Kind == token.STRING && stringLitValue(node) == invalidUsage {
				fileErrorList.
					WithLineNumber(fSet.Position(node.Pos()).Line).
					WithValue(invalidUsage).
					Errorf("Invalid certificate usage %q produces a certificate with an empty ExtendedKeyUsage "+
						"extension and is rejected by strict validators. Use %q instead.", invalidUsage, recommendedUsage)
			}
		case *ast.CallExpr:
			if isCallNamed(node, generateSelfSignedCertFunc) {
				if pos, ok := findOptionCall(node, withGroupsOption); ok {
					fileErrorList.
						WithLineNumber(fSet.Position(pos).Line).
						WithValue(withGroupsOption).
						Errorf("%s applied to a leaf certificate via %s copies the CA Organization onto the leaf, "+
							"recreating the Subject == Issuer (depth-0 self-signed) collision rejected by OpenSSL. "+
							"Remove %s from the leaf certificate.", withGroupsOption, generateSelfSignedCertFunc, withGroupsOption)
				}
			}
		}

		return true
	})
}

// fileImportsTLSCertificate reports whether the file imports the tls_certificate go_lib package.
func fileImportsTLSCertificate(astFile *ast.File) bool {
	for _, imp := range astFile.Imports {
		if imp.Path != nil && imp.Path.Value == tlsCertificateImport {
			return true
		}
	}

	return false
}

// stringLitValue returns the unquoted value of a string literal, or the raw
// value if it cannot be unquoted.
func stringLitValue(lit *ast.BasicLit) string {
	if len(lit.Value) >= 2 {
		first, last := lit.Value[0], lit.Value[len(lit.Value)-1]
		if (first == '"' && last == '"') || (first == '`' && last == '`') {
			return lit.Value[1 : len(lit.Value)-1]
		}
	}

	return lit.Value
}

// isCallNamed reports whether call invokes a function or method with the given name.
func isCallNamed(call *ast.CallExpr, name string) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel != nil && fun.Sel.Name == name
	case *ast.Ident:
		return fun.Name == name
	}

	return false
}

// findOptionCall searches the arguments of a call for a nested call to the
// option with the given name, returning its position when found.
func findOptionCall(call *ast.CallExpr, name string) (token.Pos, bool) {
	var (
		pos   token.Pos
		found bool
	)

	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			if found {
				return false
			}

			if inner, ok := n.(*ast.CallExpr); ok && isCallNamed(inner, name) {
				pos = inner.Pos()
				found = true

				return false
			}

			return true
		})
	}

	return pos, found
}
