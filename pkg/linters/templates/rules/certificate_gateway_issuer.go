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
	"os"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	CertificateGatewayIssuerRuleName = "certificate-gateway-issuer"
	recommendedGatewayIssuerInclude  = `{{ include "helm_lib_module_https_cert_manager_cluster_issuer_name_for_gateway_api" . }}`
	certificateGatewayIssuerMessage  = "Certificates related to Gateway API must refer to issuer using " +
		recommendedGatewayIssuerInclude
)

var (
	kindCertificateRegexp = regexp.MustCompile(`(?m)^\s*kind:\s*"?Certificate"?\s*$`)
	// Matches issuerRef.name set via printf "letsencrypt-gateway-%s" ... (single or double quotes).
	forbiddenGatewayIssuerPrintfRegexp = regexp.MustCompile(
		`issuerRef:\s*\n(?:[ \t]+[^\n]+\n)*?[ \t]*name:\s*\{\{[^}]*printf\s+["']letsencrypt-gateway-%s["']`,
	)
)

type CertificateGatewayIssuerRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

func NewCertificateGatewayIssuerRule(excludeRules []pkg.KindRuleExclude) *CertificateGatewayIssuerRule {
	return &CertificateGatewayIssuerRule{
		RuleMeta: pkg.RuleMeta{
			Name: CertificateGatewayIssuerRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

func (r *CertificateGatewayIssuerRule) ValidateCertificateGatewayIssuer(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	for _, object := range m.GetStorage() {
		if object.Unstructured.GetKind() != "Certificate" {
			continue
		}

		name := object.Unstructured.GetName()
		if !r.Enabled("Certificate", name) {
			continue
		}

		objectErrorList := errorList.WithObjectID(object.Identity()).WithFilePath(object.GetPath())
		content, err := os.ReadFile(object.AbsPath)
		if err != nil {
			objectErrorList.Errorf("Failed to read file %s: %v", object.GetPath(), err)
			continue
		}

		if !containsForbiddenGatewayIssuerForObject(string(content), object) {
			continue
		}

		objectErrorList.Error(certificateGatewayIssuerMessage)
	}
}

func containsForbiddenGatewayIssuerForObject(content string, object storage.StoreObject) bool {
	documents := splitYAMLDocuments(content)
	foundNamedDocument := false

	for _, document := range documents {
		if !kindCertificateRegexp.MatchString(document) {
			continue
		}

		if !documentContainsObjectName(document, object.Unstructured.GetName()) {
			continue
		}

		foundNamedDocument = true
		if containsForbiddenGatewayIssuer(document) {
			return true
		}
	}

	if foundNamedDocument {
		return false
	}

	return containsForbiddenGatewayIssuer(content)
}

func containsForbiddenGatewayIssuer(content string) bool {
	if !kindCertificateRegexp.MatchString(content) {
		return false
	}

	if forbiddenGatewayIssuerPrintfRegexp.MatchString(content) {
		return true
	}

	// Fallback for single-line / differently indented issuerRef.name forms.
	return strings.Contains(content, `issuerRef`) &&
		(strings.Contains(content, `printf "letsencrypt-gateway-%s"`) ||
			strings.Contains(content, `printf 'letsencrypt-gateway-%s'`))
}

func splitYAMLDocuments(content string) []string {
	return regexp.MustCompile(`(?m)^---\s*$`).Split(content, -1)
}

func documentContainsObjectName(document, objectName string) bool {
	quotedName := regexp.QuoteMeta(objectName)
	nameRegexp := regexp.MustCompile(`(?m)^\s*name:\s*"?` + quotedName + `"?\s*$`)

	return nameRegexp.MatchString(document)
}
