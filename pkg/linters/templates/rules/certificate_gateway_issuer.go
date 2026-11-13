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
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	CertificateGatewayIssuerRuleName = "certificate-gateway-issuer"

	gatewayAPIClusterIssuerHelper = "helm_lib_module_https_cert_manager_cluster_issuer_name_for_gateway_api"
	// gatewayAPIClusterIssuerHelperLibHelm is the first lib_helm release that ships gatewayAPIClusterIssuerHelper.
	gatewayAPIClusterIssuerHelperLibHelm = "v1.72.10"
	clusterIssuerHelper                  = "helm_lib_module_https_cert_manager_cluster_issuer_name"

	// letsencryptIssuer is the substring every built-in letsencrypt ClusterIssuer name
	// (letsencrypt, letsencrypt-staging, letsencrypt-gateway-<name>, ...) shares.
	letsencryptIssuer = "letsencrypt"

	certificateGatewayIssuerMessage = "Certificate issuerRef.name is pinned to the letsencrypt ClusterIssuer, " +
		"so the ClusterIssuer configured in https.certManager.clusterIssuerName (globally or for the module) " +
		"is never used: for Gateway API use {{ include \"" + gatewayAPIClusterIssuerHelper + "\" . }} " +
		"(requires lib_helm " + gatewayAPIClusterIssuerHelperLibHelm + "+), or take " +
		"{{ include \"" + clusterIssuerHelper + "\" . }} and add the \"-gateway-<gateway name>\" suffix only " +
		"inside {{- if or (eq $issuerName \"letsencrypt\") (eq $issuerName \"letsencrypt-staging\") }}; " +
		"for Ingress use {{ include \"" + clusterIssuerHelper + "\" . }}. Both helpers fail to render unless " +
		"the https mode is CertManager, so keep the Certificate under " +
		"{{- if eq (include \"helm_lib_module_https_mode\" .) \"CertManager\" }}"
)

var (
	certificateKindLineRe = regexp.MustCompile(`^\s*kind:\s*["']?Certificate["']?\s*$`)
	documentSeparatorRe   = regexp.MustCompile(`^---(\s.*)?$`)
	issuerRefLineRe       = regexp.MustCompile(`^(\s*)issuerRef:\s*(.*)$`)
	metadataLineRe        = regexp.MustCompile(`^(\s*)metadata:\s*$`)
	nameLineRe            = regexp.MustCompile(`^(\s*)name:\s*(.*)$`)

	// templateCommentRe matches a Helm comment action, {{/* ... */}} with optional trim markers.
	templateCommentRe = regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)
	// templateAssignmentRe matches a variable declaration or assignment inside an action body
	// (templateActionRe's capture): `$issuerName := ...` or `$issuerName = ...`. The right-hand
	// side runs to the end of the action.
	templateAssignmentRe = regexp.MustCompile(`(\$[A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*(.*)$`)
	templateVariableRe   = regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*`)
)

// CertificateGatewayIssuerRule forbids Certificate templates from pinning issuerRef.name to a
// letsencrypt ClusterIssuer: a hardcoded name (letsencrypt, letsencrypt-gateway-<name>, a
// printf "letsencrypt-gateway-%s", ...) ignores the issuer the cluster or the module configures
// in https.certManager.clusterIssuerName.
//
// The rule scans the raw template text instead of the rendered objects, because Certificates
// are usually wrapped in Helm conditionals (https mode, gateway or ingress enabled) that the
// default values used for linting do not satisfy, so they never reach the rendered storage.
type CertificateGatewayIssuerRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewCertificateGatewayIssuerRule(excludeRules []pkg.KindRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *CertificateGatewayIssuerRule {
	return &CertificateGatewayIssuerRule{
		RuleMeta: pkg.RuleMeta{
			Name: CertificateGatewayIssuerRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(CertificateGatewayIssuerRuleName),
	}
}

var _ pkg.Rule = (*CertificateGatewayIssuerRule)(nil)

func (r *CertificateGatewayIssuerRule) Check(_ context.Context) {
	modulePath := r.module.GetPath()

	templatesPath := filepath.Join(modulePath, "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return
	}

	files := fsutils.GetFiles(templatesPath, true, fsutils.FilterFileByExtensions(".yaml", ".yml", ".tpl"))

	for _, filePath := range files {
		relPath := fsutils.Rel(modulePath, filePath)

		content, err := os.ReadFile(filePath)
		if err != nil {
			r.errorList.WithFilePath(relPath).Errorf("Failed to read file: %v", err)
			continue
		}

		for _, finding := range findHardcodedLetsencryptIssuers(string(content)) {
			if !r.Enabled("Certificate", finding.certificateName) {
				continue
			}

			r.errorList.
				WithFilePath(relPath).
				WithLineNumber(finding.line).
				WithObjectID("kind = Certificate ; name = " + finding.certificateName).
				WithValue(finding.value).
				Error(certificateGatewayIssuerMessage)
		}
	}
}

// issuerFinding is a Certificate issuerRef.name that resolves to a hardcoded letsencrypt issuer.
type issuerFinding struct {
	// certificateName is metadata.name of the Certificate as written in the template.
	certificateName string
	// line is the 1-based line of the offending issuerRef.name.
	line int
	// value is the offending text: the issuerRef.name value, or the variable assignment it uses.
	value string
}

// templateAssignment is a `$var := ...` / `$var = ...` found in a template file.
type templateAssignment struct {
	variable string
	rhs      string
	// text is the whole assignment as written, for reporting.
	text   string
	offset int
}

// findHardcodedLetsencryptIssuers returns every issuerRef.name of every Certificate in a raw
// template file that hardcodes a letsencrypt ClusterIssuer, either directly in the value or
// through a template variable assigned a letsencrypt literal earlier in the file. Helm
// conditionals are ignored on purpose, so Certificates that are not rendered with the default
// values are checked too.
func findHardcodedLetsencryptIssuers(content string) []issuerFinding {
	// Blank Helm comments out, keeping offsets and line numbers intact.
	content = templateCommentRe.ReplaceAllStringFunc(content, blankKeepingNewlines)

	if !strings.Contains(content, letsencryptIssuer) {
		return nil
	}

	assignments := collectTemplateAssignments(content)

	lines := strings.Split(content, "\n")

	lineOffsets := make([]int, len(lines))
	for i, offset := 1, 0; i < len(lines); i++ {
		offset += len(lines[i-1]) + 1
		lineOffsets[i] = offset
	}

	var findings []issuerFinding

	for _, doc := range splitDocuments(lines) {
		if !documentIsCertificate(lines[doc.start:doc.end]) {
			continue
		}

		certificateName := documentMetadataName(lines[doc.start:doc.end])

		for _, ref := range documentIssuerNames(lines, doc) {
			value, ok := hardcodedLetsencryptIssuer(ref.value, assignments, lineOffsets[ref.line])
			if !ok {
				continue
			}

			findings = append(findings, issuerFinding{
				certificateName: certificateName,
				line:            ref.line + 1,
				value:           value,
			})
		}
	}

	return findings
}

// hardcodedLetsencryptIssuer reports whether an issuerRef.name value pins a letsencrypt issuer
// and returns the text that does it. A value referencing template variables is resolved through
// the assignments made before it (offset), following variable-to-variable assignments.
func hardcodedLetsencryptIssuer(value string, assignments []templateAssignment, offset int) (string, bool) {
	value = stripYAMLComment(value)

	if strings.Contains(value, letsencryptIssuer) {
		return value, true
	}

	visited := map[string]bool{}

	var resolve func(text string, before int) (string, bool)

	resolve = func(text string, before int) (string, bool) {
		for _, variable := range templateVariableRe.FindAllString(text, -1) {
			if visited[variable] {
				continue
			}

			visited[variable] = true

			for _, assignment := range assignments {
				if assignment.variable != variable || assignment.offset >= before {
					continue
				}

				if strings.Contains(assignment.rhs, letsencryptIssuer) {
					return assignment.text, true
				}

				if found, ok := resolve(assignment.rhs, assignment.offset); ok {
					return found, true
				}
			}
		}

		return "", false
	}

	var actions strings.Builder
	for _, match := range templateActionRe.FindAllStringSubmatch(value, -1) {
		actions.WriteString(match[1])
		actions.WriteByte(' ')
	}

	return resolve(actions.String(), offset)
}

// collectTemplateAssignments returns every variable declaration and assignment in the file.
func collectTemplateAssignments(content string) []templateAssignment {
	var assignments []templateAssignment

	for _, loc := range templateActionRe.FindAllStringSubmatchIndex(content, -1) {
		match := templateAssignmentRe.FindStringSubmatch(content[loc[2]:loc[3]])
		if match == nil {
			continue
		}

		assignments = append(assignments, templateAssignment{
			variable: match[1],
			rhs:      strings.TrimSpace(match[2]),
			text:     strings.TrimSpace(match[0]),
			offset:   loc[0],
		})
	}

	return assignments
}

type lineRange struct {
	start, end int
}

// splitDocuments splits template lines into YAML documents on `---` separator lines.
func splitDocuments(lines []string) []lineRange {
	var docs []lineRange

	start := 0

	for i, line := range lines {
		if documentSeparatorRe.MatchString(line) {
			docs = append(docs, lineRange{start: start, end: i})
			start = i + 1
		}
	}

	return append(docs, lineRange{start: start, end: len(lines)})
}

func documentIsCertificate(lines []string) bool {
	for _, line := range lines {
		if certificateKindLineRe.MatchString(line) {
			return true
		}
	}

	return false
}

// documentMetadataName returns metadata.name of a document as written in the template.
func documentMetadataName(lines []string) string {
	for i, line := range lines {
		match := metadataLineRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		for _, child := range blockChildren(lines[i+1:], len(match[1])) {
			if nameMatch := nameLineRe.FindStringSubmatch(child); nameMatch != nil {
				return strings.Trim(stripYAMLComment(nameMatch[2]), `"'`)
			}
		}

		return ""
	}

	return ""
}

type issuerNameRef struct {
	// line is the 0-based line index of the issuerRef.name (or inline issuerRef) value.
	line  int
	value string
}

// documentIssuerNames returns every issuerRef.name value of a document. Names nested in Helm
// conditionals inside the issuerRef block are all returned, as is an inline issuerRef value.
func documentIssuerNames(lines []string, doc lineRange) []issuerNameRef {
	var refs []issuerNameRef

	for i := doc.start; i < doc.end; i++ {
		match := issuerRefLineRe.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}

		if inline := stripYAMLComment(match[2]); inline != "" {
			refs = append(refs, issuerNameRef{line: i, value: inline})
			continue
		}

		indent := len(match[1])

		for j := i + 1; j < doc.end; j++ {
			line := lines[j]
			trimmed := strings.TrimSpace(line)

			if trimmed == "" || strings.HasPrefix(trimmed, "#") || isTemplateOnlyLine(trimmed) {
				continue
			}

			if leadingSpaces(line) <= indent {
				break
			}

			if nameMatch := nameLineRe.FindStringSubmatch(line); nameMatch != nil {
				refs = append(refs, issuerNameRef{line: j, value: nameMatch[2]})
			}
		}
	}

	return refs
}

// blockChildren returns the lines nested deeper than indent, up to the end of the block.
func blockChildren(lines []string, indent int) []string {
	var children []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") || isTemplateOnlyLine(trimmed) {
			continue
		}

		if leadingSpaces(line) <= indent {
			break
		}

		children = append(children, line)
	}

	return children
}

// isTemplateOnlyLine reports whether a trimmed line holds nothing but Helm actions, such as
// `{{- if ... }}` or `{{- end }}`, which do not take part in the YAML indentation.
func isTemplateOnlyLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") &&
		strings.TrimSpace(templateActionRe.ReplaceAllString(trimmed, "")) == ""
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// stripYAMLComment drops a trailing `# comment` that is outside Helm actions and quotes.
func stripYAMLComment(value string) string {
	var (
		depth int
		quote byte
	)

	for i := 0; i < len(value); i++ {
		switch c := value[i]; {
		case strings.HasPrefix(value[i:], "{{"):
			depth++
			i++
		case strings.HasPrefix(value[i:], "}}") && depth > 0:
			depth--
			i++
		case depth > 0:
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#' && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t'):
			return strings.TrimSpace(value[:i])
		}
	}

	return strings.TrimSpace(value)
}

func blankKeepingNewlines(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}

		return ' '
	}, s)
}
