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

package module

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/helm"
	"github.com/deckhouse/dmt/internal/storage"
	dmtErrors "github.com/deckhouse/dmt/pkg/errors"
)

func RunRender(m *Module, values chartutil.Values, objectStore *storage.UnstructuredObjectStore, errorList *dmtErrors.LintRuleErrorsList) error {
	renderer := helm.Renderer{
		Name:      m.GetName(),
		Namespace: m.GetNamespace(),
		LintMode:  true,
		// Inject dmt's deterministic helm_lib helper stubs so image and
		// module-name references resolve to stable values during linting.
		HelmLibOverrides: helmLibOverrides(),
	}

	files, err := renderer.RenderChartFromDir(m.GetPath(), values)
	if err != nil {
		return fmt.Errorf("helm chart render: %w", err)
	}

	var resultErr error

	for path, bigFile := range files {
		// path example: module/templates/file.yaml
		// short path example: templates/file.yaml
		shortPath := path

		elements := strings.Split(path, string(os.PathSeparator))
		if len(elements) > 0 {
			shortPath = strings.Join(elements[1:], string(os.PathSeparator))
		}

		absPath, err := filepath.Abs(filepath.Join(m.path, shortPath))
		if err != nil {
			// TODO: handle error
			_ = err
		}

		if absPath != "" {
			path = absPath
		}

		docs := splitYAMLDocuments(bigFile)
		for _, doc := range docs {
			docBytes := []byte(doc)
			if len(docBytes) == 0 {
				continue
			}

			node := make(map[string]any)

			err = yaml.UnmarshalStrict(docBytes, &node)
			if err != nil {
				// dmt feeds auto-generated placeholder values into the chart when
				// linting. A string value that a template decodes (e.g. via
				// `b64dec`/`b32dec`) then turns into arbitrary binary bytes, which
				// renders a manifest that is not valid UTF-8 / contains control
				// characters. That is an artifact of the synthetic values, not a
				// module defect. Replace the offending bytes with a printable
				// placeholder so the manifest stays parseable and is still linted,
				// instead of dropping the whole module. Genuine structural YAML
				// errors are still reported.
				if !isBinaryManifest(docBytes) {
					return fmt.Errorf(manifestErrorMessage, strings.TrimPrefix(path, m.GetName()+"/"), err)
				}

				sanitized := sanitizeBinaryManifest(docBytes)

				node = make(map[string]any)
				if retryErr := yaml.UnmarshalStrict(sanitized, &node); retryErr != nil {
					// Sanitizing did not yield a parseable manifest (e.g. decoded
					// bytes happened to form YAML control characters). Skip the
					// document rather than dropping the whole module.
					log.Debug("skipping rendered manifest with unrecoverable binary content",
						slog.String("path", strings.TrimPrefix(path, m.GetName()+"/")),
						slog.String("error", err.Error()),
					)

					resultErr = errors.Join(resultErr, retryErr)

					continue
				}

				docBytes = sanitized
			}

			if len(node) == 0 {
				continue
			}

			err = objectStore.Put(path, shortPath, node, docBytes)
			if err != nil {
				resultErr = errors.Join(resultErr, err)
				continue
			}
		}
	}

	if resultErr != nil {
		errorList.WithFilePath(m.GetPath()).WithModule(m.GetName()).
			WithValue(resultErr.Error()).Error("module contains duplicate objects")
	}

	return nil
}

const (
	manifestErrorMessage = `manifest (%q) unmarshal: %v`
)

// isBinaryManifest reports whether the rendered manifest contains bytes that can
// never appear in a valid YAML manifest: invalid UTF-8 sequences or disallowed
// control characters. Such content is produced when dmt's synthetic placeholder
// values are decoded by templates (e.g. via `b64dec`/`b32dec`) into arbitrary
// binary data.
func isBinaryManifest(b []byte) bool {
	if !utf8.Valid(b) {
		return true
	}

	for _, r := range string(b) {
		switch r {
		case '\t', '\n', '\r':
			continue
		}

		if r < 0x20 || r == 0x7f {
			return true
		}
	}

	return false
}

// binaryPlaceholderByte replaces bytes that cannot appear in a valid YAML
// manifest. It is intentionally a YAML-neutral printable character so the
// surrounding document structure is preserved.
const binaryPlaceholderByte = '_'

// sanitizeBinaryManifest returns a copy of b with invalid UTF-8 bytes and
// disallowed control characters replaced by binaryPlaceholderByte. Tabs,
// newlines and carriage returns are preserved so the document layout (and thus
// its YAML structure) stays intact. This lets dmt keep linting manifests whose
// values were decoded from synthetic placeholders into binary data, rather than
// discarding them.
func sanitizeBinaryManifest(b []byte) []byte {
	out := make([]byte, 0, len(b))

	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])

		switch {
		case r == utf8.RuneError && size <= 1:
			out = append(out, binaryPlaceholderByte)
			i++
		case r == '\t' || r == '\n' || r == '\r':
			out = append(out, b[i:i+size]...)
			i += size
		case r < 0x20 || r == 0x7f:
			out = append(out, binaryPlaceholderByte)
			i += size
		default:
			out = append(out, b[i:i+size]...)
			i += size
		}
	}

	return out
}

func splitYAMLDocuments(content string) []string {
	var (
		docs    []string
		current strings.Builder
	)

	lines := strings.Split(content, "\n")
	inMultiLine := false
	indentLevel := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're starting a multi-line value.
		if !inMultiLine && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") {
					inMultiLine = true
					indentLevel = len(line) - len(strings.TrimLeft(line, " \t"))
				}
			}
		}

		// Check if we're ending a multi-line value
		if inMultiLine {
			currentLineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if currentLineIndent <= indentLevel && trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				inMultiLine = false
				indentLevel = 0
			}
		}

		// Check for document separator (---) only when not in multi-line
		if !inMultiLine && trimmed == "---" {
			if current.Len() > 0 {
				docs = append(docs, strings.TrimSpace(current.String()))
				current.Reset()
			}

			continue
		}

		// Add line to current document
		if current.Len() > 0 {
			current.WriteString("\n")
		}

		current.WriteString(line)
	}

	// Add the last document
	if current.Len() > 0 {
		docs = append(docs, strings.TrimSpace(current.String()))
	}

	return docs
}
