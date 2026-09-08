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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	HTTPSCertificateReuseRuleName = "https-certificate-reuse"
)

var (
	// httpsCopyCustomCertificateRe matches a call to
	// helm_lib_module_https_copy_custom_certificate, capturing its third
	// argument (secret_name_prefix). The second argument (namespace) is not
	// captured; it may be a quoted literal or a bare Helm expression.
	httpsCopyCustomCertificateRe = regexp.MustCompile(
		`helm_lib_module_https_copy_custom_certificate"\s*\(\s*list\s+\S+\s+(?:"[^"]*"|\S+)\s+"([^"]+)"\s*\)`,
	)

	// httpsSecretNameRe matches a call to helm_lib_module_https_secret_name,
	// capturing its secret_name_prefix (always present) and, when the
	// two-prefix form is used, the Gateway-API-specific override prefix.
	httpsSecretNameRe = regexp.MustCompile(
		`helm_lib_module_https_secret_name"\s*\(\s*list\s+\S+\s+"([^"]+)"(?:\s+"([^"]+)")?\s*\)`,
	)

	// yamlDocumentSeparatorRe matches a "---" document separator on its own
	// line, the convention templates use to emit multiple manifests from one
	// file.
	yamlDocumentSeparatorRe = regexp.MustCompile(`(?m)^---[ \t]*$`)
)

// certFlowSuffixes are the conventional customcertificate secret_name_prefix
// suffixes used throughout the module ecosystem: a module's Ingress-flow copy
// is commonly named "<stem>-ingress-tls" (or bare "ingress-tls") and its
// Gateway API/HTTPRoute-flow copy "<stem>-httproute-tls" (or bare
// "httproute-tls").
var certFlowSuffixes = map[string]string{
	"ingress-tls":   "ingress",
	"httproute-tls": "httproute",
}

// certFlowStem splits a customcertificate secret_name_prefix into its logical
// service stem and flow (ingress or httproute), if it follows that naming
// convention. "istio-httproute-tls" -> ("istio", "httproute", true);
// "httproute-tls" -> ("", "httproute", true); "service-a-tls" -> ("", "", false).
func certFlowStem(prefix string) (string, string, bool) {
	for suffix, f := range certFlowSuffixes {
		if prefix == suffix {
			return "", f, true
		}

		if trimmed, found := strings.CutSuffix(prefix, "-"+suffix); found {
			return trimmed, f, true
		}
	}

	return "", "", false
}

// splitYAMLDocuments returns the [start, end) byte ranges of each YAML
// document in content, split on "---" document separators. A file with no
// separator is a single document spanning the whole content.
func splitYAMLDocuments(content []byte) [][2]int {
	seps := yamlDocumentSeparatorRe.FindAllIndex(content, -1)
	if len(seps) == 0 {
		return [][2]int{{0, len(content)}}
	}

	docs := make([][2]int, 0, len(seps)+1)
	start := 0

	for _, sep := range seps {
		docs = append(docs, [2]int{start, sep[0]})
		start = sep[1]
	}

	docs = append(docs, [2]int{start, len(content)})

	return docs
}

// httpsCertCopy is one occurrence of helm_lib_module_https_copy_custom_certificate.
type httpsCertCopy struct {
	prefix  string
	relPath string
	line    int
}

// httpsSecretRef is one occurrence of helm_lib_module_https_secret_name.
// linked is true for the two-prefix form (list . base override); for the
// plain one-prefix form, override is empty and linked is false.
type httpsSecretRef struct {
	base     string
	override string
	linked   bool
	relPath  string
	line     int
}

type HTTPSCertificateReuseRule struct {
	pkg.RuleMeta
	pkg.PathRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewHTTPSCertificateReuseRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *HTTPSCertificateReuseRule {
	return &HTTPSCertificateReuseRule{
		RuleMeta: pkg.RuleMeta{
			Name: HTTPSCertificateReuseRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
		},
		module:    m,
		errorList: errorList.WithRule(HTTPSCertificateReuseRuleName),
	}
}

var _ pkg.Rule = (*HTTPSCertificateReuseRule)(nil)

// Check enforces that a module serving HTTPS over both Ingress and Gateway
// API copies its CustomCertificate-mode certificate exactly once per secret,
// and that both flows reuse that one copy via
// helm_lib_module_https_secret_name: the Ingress manifest with the plain,
// one-prefix form ({{ include "helm_lib_module_https_secret_name" (list . "base") }}),
// the HTTPRoute/ListenerSet manifest with the two-prefix form
// ({{ include "helm_lib_module_https_secret_name" (list . "base" "override") }}),
// linking back to the exact same "base". In CertManager mode the two-prefix
// form resolves "override" to its own secret (Gateway API needs its own
// cert-manager Certificate, validated through a separate ClusterIssuer), but
// in CustomCertificate mode it ignores the override and resolves to "base"'s
// secret instead — so the Gateway API flow never needs, and must never copy,
// a second CustomCertificate secret of its own.
//
// This is a textual, whole-module heuristic scanning every template file for
// helm_lib_module_https_copy_custom_certificate and
// helm_lib_module_https_secret_name calls, classifying each
// helm_lib_module_https_secret_name call by the kind(s) declared in its own
// YAML document — a file is split on "---" separators first (see
// splitYAMLDocuments), since one file commonly bundles a HTTPRoute/ListenerSet
// alongside the cert-manager Certificate that feeds it. A call inside a
// `kind: Certificate` document is excluded entirely: a Certificate's own
// secretName always uses the plain form to declare a new target secret for
// cert-manager to populate, which isn't the Ingress or Gateway API manifest
// "consuming" a shared secret that this rule is about. Otherwise a call
// counts as an Ingress or Gateway API reference if its document declares
// `kind: Ingress` or `kind: HTTPRoute`/`kind: ListenerSet` respectively. It
// reports, deduplicated by location:
//
//  1. A secret_name_prefix copied by more than one
//     helm_lib_module_https_copy_custom_certificate call — only one copy is
//     ever needed for a given secret.
//  2. An Ingress-file reference using the two-prefix form — Ingress has no
//     use for a Gateway-API-specific override, since it never needs to
//     diverge from the shared secret.
//  3. A HTTPRoute/ListenerSet-file reference using the plain one-prefix
//     form — without the override, that manifest names its own,
//     independent secret instead of reusing the Ingress flow's.
//  4. A two-prefix form's override that is ALSO independently copied — that
//     copy is unreachable, since the link already made the base's copy
//     available to the Gateway API flow in CustomCertificate mode.
//  5. Two copied prefixes follow the "<stem>-ingress-tls" / "<stem>-httproute-tls"
//     naming convention for the same stem (see certFlowStem). Unlike checks
//     1-4, this one fires purely from the copy calls themselves, so it also
//     catches a module that copies both variants but hasn't wired up (or has
//     wired up incorrectly) either flow's reference yet; it runs last and
//     only adds a finding where none of the above already reported the
//     exact same location.
//
// The rule only runs when the module renders both an Ingress and a
// HTTPRoute/ListenerSet: reuse across flows is only possible when both flows
// exist.
func (r *HTTPSCertificateReuseRule) Check(_ context.Context) {
	m := r.module

	if !storageHasKind(m, "Ingress") || !storageHasKind(m, "HTTPRoute", "ListenerSet") {
		return
	}

	templatesPath := filepath.Join(m.GetPath(), "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	files := fsutils.GetFiles(templatesPath, true, fsutils.FilterFileByExtensions(".yaml", ".yml", ".tpl"))
	ingressKindRe := kindLineRe("Ingress")
	gatewayKindRe := kindLineRe("HTTPRoute", "ListenerSet")
	certificateKindRe := kindLineRe("Certificate")

	var copies []httpsCertCopy

	var ingressRefs []httpsSecretRef

	var gatewayRefs []httpsSecretRef

	for _, filePath := range files {
		relPath := fsutils.Rel(m.GetPath(), filePath)

		if !r.Enabled(relPath) {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			r.errorList.WithFilePath(relPath).Errorf("Failed to read file: %v", err)
			continue
		}

		for _, loc := range httpsCopyCustomCertificateRe.FindAllSubmatchIndex(content, -1) {
			copies = append(copies, httpsCertCopy{
				prefix:  string(content[loc[2]:loc[3]]),
				relPath: relPath,
				line:    bytes.Count(content[:loc[0]], []byte("\n")) + 1,
			})
		}

		// A file often bundles several manifests (e.g. a HTTPRoute alongside
		// the cert-manager Certificate that feeds it) separated by "---", so
		// classify each helm_lib_module_https_secret_name call by the kind(s)
		// declared in its own document, not by every kind anywhere in the
		// file. A Certificate's own secretName always uses the plain form —
		// it's declaring a new target secret for cert-manager to populate,
		// not an Ingress or Gateway API manifest consuming an existing one —
		// so calls inside a Certificate document are excluded entirely.
		for _, doc := range splitYAMLDocuments(content) {
			docContent := content[doc[0]:doc[1]]

			if certificateKindRe.Match(docContent) {
				continue
			}

			isIngressDoc := ingressKindRe.Match(docContent)
			isGatewayDoc := gatewayKindRe.Match(docContent)

			if !isIngressDoc && !isGatewayDoc {
				continue
			}

			for _, loc := range httpsSecretNameRe.FindAllSubmatchIndex(docContent, -1) {
				ref := httpsSecretRef{
					base:    string(docContent[loc[2]:loc[3]]),
					linked:  loc[4] >= 0,
					relPath: relPath,
					line:    bytes.Count(content[:doc[0]+loc[0]], []byte("\n")) + 1,
				}
				if ref.linked {
					ref.override = string(docContent[loc[4]:loc[5]])
				}

				if isIngressDoc {
					ingressRefs = append(ingressRefs, ref)
				}

				if isGatewayDoc {
					gatewayRefs = append(gatewayRefs, ref)
				}
			}
		}
	}

	copiedAt := make(map[string][]httpsCertCopy, len(copies))

	for _, c := range copies {
		copiedAt[c.prefix] = append(copiedAt[c.prefix], c)
	}

	reported := make(map[string]bool)
	reportOnce := func(relPath string, line int, value, format string, args ...any) {
		key := fmt.Sprintf("%s:%d", relPath, line)
		if reported[key] {
			return
		}

		reported[key] = true

		r.errorList.WithFilePath(relPath).
			WithLineNumber(line).
			WithValue(value).
			Errorf(format, args...)
	}

	// 1. A secret copied by more than one helm_lib_module_https_copy_custom_certificate call.
	for _, locs := range copiedAt {
		if len(locs) < 2 {
			continue
		}

		for _, dupe := range locs[1:] {
			reportOnce(dupe.relPath, dupe.line, dupe.prefix,
				"Secret prefix %q is copied by more than one helm_lib_module_https_copy_custom_certificate "+
					"call (also at %s:%d) — only one copy is needed for a given secret; every consumer should "+
					"reuse it via helm_lib_module_https_secret_name instead of copying it again.",
				dupe.prefix, locs[0].relPath, locs[0].line,
			)
		}
	}

	// 2. An Ingress-file reference using the two-prefix (Gateway-API-override) form.
	for _, ref := range ingressRefs {
		if !ref.linked {
			continue
		}

		reportOnce(ref.relPath, ref.line, ref.base,
			"Ingress references its TLS secret with the two-prefix form of helm_lib_module_https_secret_name "+
				"(list . %q %q), but Ingress has no Gateway-API-specific override to apply — use the plain "+
				"form (list . %q) instead.",
			ref.base, ref.override, ref.base,
		)
	}

	// 3. A HTTPRoute/ListenerSet-file reference using the plain one-prefix form.
	for _, ref := range gatewayRefs {
		if ref.linked {
			continue
		}

		reportOnce(ref.relPath, ref.line, ref.base,
			"HTTPRoute/ListenerSet references its TLS secret with the plain form of "+
				"helm_lib_module_https_secret_name (list . %[1]q). It is recommended to use the extended form "+
				"instead (list . \"<ingress-secret-prefix>\" %[1]q), which allows reusing the custom certificate "+
				"secret in case CustomCertificate mode is enabled.",
			ref.base,
		)
	}

	// 4. A two-prefix reference whose override is also independently copied.
	for _, ref := range gatewayRefs {
		if !ref.linked {
			continue
		}

		for _, dupe := range copiedAt[ref.override] {
			reportOnce(dupe.relPath, dupe.line, dupe.prefix,
				"File copies a custom certificate under secret prefix %q, but %q is only meant to be the "+
					"Gateway-API-specific override in the two-prefix form of helm_lib_module_https_secret_name "+
					"(list . %q %q), found in %s:%d. Under CustomCertificate mode both the Ingress and Gateway "+
					"API flows already resolve to %q's copy, so copying one under %q too is a duplicate — "+
					"remove this helm_lib_module_https_copy_custom_certificate call and let the two-prefix "+
					"form share %q's certificate.",
				dupe.prefix, ref.override, ref.base, ref.override, ref.relPath, ref.line,
				ref.base, ref.override, ref.base,
			)
		}
	}

	// 5. Two copied prefixes follow the "<stem>-ingress-tls" / "<stem>-httproute-tls"
	//    naming convention for the same stem. Runs last and only adds a
	//    finding where none of the checks above already reported this exact
	//    location: unlike them, it fires purely from the copy calls
	//    themselves, so it also catches a module that copies both variants
	//    but hasn't wired up (or has wired up incorrectly) either flow's
	//    reference yet, which the reference-based checks above cannot see.
	ingressCopyByStem := make(map[string]httpsCertCopy)

	for _, c := range copies {
		if stem, flow, ok := certFlowStem(c.prefix); ok && flow == "ingress" {
			if _, seen := ingressCopyByStem[stem]; !seen {
				ingressCopyByStem[stem] = c
			}
		}
	}

	for _, c := range copies {
		stem, flow, ok := certFlowStem(c.prefix)
		if !ok || flow != "httproute" {
			continue
		}

		ingressCopy, hasIngress := ingressCopyByStem[stem]
		if !hasIngress {
			continue
		}

		reportOnce(c.relPath, c.line, c.prefix,
			"File copies a custom certificate under secret prefix %q, which by naming convention is the "+
				"Gateway API/HTTPRoute variant of %q — also copied via helm_lib_module_https_copy_custom_certificate, "+
				"in %s:%d. Copy the certificate once, under %q, and reference it from the Gateway API flow with the "+
				"two-prefix form of helm_lib_module_https_secret_name (list . %q %q) instead of copying it separately.",
			c.prefix, ingressCopy.prefix, ingressCopy.relPath, ingressCopy.line,
			ingressCopy.prefix, ingressCopy.prefix, c.prefix,
		)
	}
}
