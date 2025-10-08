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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// License represents a license type with its template
type License struct {
	Type        string // "CE" or "EE"
	Name        string // Human-readable name
	Template    string // License template with {{YEAR}} placeholder
	YearPattern string // Regex pattern for year validation
}

// CommentStyle defines how comments are formatted in different file types
type CommentStyle struct {
	LinePrefix string // Prefix for single-line comments (e.g., "//", "#")
	BlockStart string // Start of block comment (e.g., "/*", "<!--")
	BlockEnd   string // End of block comment (e.g., "*/", "-->")
	BlockLine  string // Optional prefix for lines within block (e.g., " * ")
}

// FileTypeConfig defines comment styles for specific file types
type FileTypeConfig struct {
	Extensions    []string       // File extensions (e.g., ".go", ".py")
	CommentStyles []CommentStyle // Supported comment styles
}

// LicenseInfo contains information about parsed license
type LicenseInfo struct {
	Type  string // "CE", "EE", or empty
	Year  string // Extracted year
	Valid bool   // Whether license is valid
	Error string // Error message if invalid
}

// LicenseParser handles license parsing and validation
type LicenseParser struct {
	licenses    []License
	fileConfigs map[string]FileTypeConfig
}

// NewLicenseParser creates a new license parser with default configuration
func NewLicenseParser() *LicenseParser {
	return &LicenseParser{
		licenses: []License{
			{
				Type: "CE",
				Name: "Apache License 2.0",
				Template: `Copyright {{YEAR}} Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.`,
				YearPattern: `20[0-9]{2}`,
			},
			{
				Type: "CE",
				Name: "Apache License 2.0 Modified",
				Template: `Copyright {{YEAR}} Flant JSC

Modifications made by Flant JSC as part of the Deckhouse project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.`,
				YearPattern: `20[0-9]{2}`,
			},
			{
				Type: "CE",
				Name: "SPDX Apache-2.0",
				Template: `Copyright (c) Flant JSC.
SPDX-License-Identifier: Apache-2.0`,
			},
			{
				Type: "EE",
				Name: "Deckhouse Platform Enterprise Edition",
				Template: `Copyright {{YEAR}} Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE`,
				YearPattern: `20[0-9]{2}`,
			},
		},
		fileConfigs: initFileConfigs(),
	}
}

// initFileConfigs initializes file type configurations
func initFileConfigs() map[string]FileTypeConfig {
	configs := make(map[string]FileTypeConfig)

	// Go files
	configs[".go"] = FileTypeConfig{
		Extensions: []string{".go"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "//"},
			{BlockStart: "/*", BlockEnd: "*/"},
		},
	}

	// Shell scripts
	configs[".sh"] = FileTypeConfig{
		Extensions: []string{".sh", ".bash"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "#"},
		},
	}

	// Python files
	configs[".py"] = FileTypeConfig{
		Extensions: []string{".py"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "#"},
			{BlockStart: `"""`, BlockEnd: `"""`},
			{BlockStart: `'''`, BlockEnd: `'''`},
		},
	}

	// Lua files
	configs[".lua"] = FileTypeConfig{
		Extensions: []string{".lua"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "--"},
			{BlockStart: "--[[", BlockEnd: "--]]"},
		},
	}

	// YAML files
	configs[".yaml"] = FileTypeConfig{
		Extensions: []string{".yaml", ".yml"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "#"},
		},
	}

	// Dockerfile
	configs["Dockerfile"] = FileTypeConfig{
		Extensions: []string{"Dockerfile"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "#"},
		},
	}

	// JavaScript/TypeScript
	configs[".js"] = FileTypeConfig{
		Extensions: []string{".js", ".ts", ".jsx", ".tsx"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "//"},
			{BlockStart: "/*", BlockEnd: "*/"},
		},
	}

	// C/C++
	configs[".c"] = FileTypeConfig{
		Extensions: []string{".c", ".h", ".cpp", ".hpp", ".cc", ".hh"},
		CommentStyles: []CommentStyle{
			{LinePrefix: "//"},
			{BlockStart: "/*", BlockEnd: "*/"},
		},
	}

	return configs
}

// ParseFile parses a file and extracts license information
func (p *LicenseParser) ParseFile(filename string) (*LicenseInfo, error) {
	// Get file type configuration
	config := p.getFileConfig(filename)
	if config == nil {
		return &LicenseInfo{
			Valid: false,
			Error: fmt.Sprintf("unsupported file type: %s", filename),
		}, nil
	}

	// Read file header
	const maxHeaderSize = 2048
	header, err := p.readFileHeader(filename, maxHeaderSize) // Read more bytes for better detection
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to extract license text from header
	licenseText := p.extractLicenseText(header, config)
	if licenseText == "" {
		return &LicenseInfo{
			Valid: false,
			Error: "no license header found",
		}, nil
	}

	// Match against known licenses
	for _, license := range p.licenses {
		if matched, year := p.matchLicense(licenseText, license); matched {
			return &LicenseInfo{
				Type:  license.Type,
				Year:  year,
				Valid: true,
			}, nil
		}
	}

	return &LicenseInfo{
		Valid: false,
		Error: "license header does not match any known license",
	}, nil
}

// getFileConfig returns the configuration for a given file
func (p *LicenseParser) getFileConfig(filename string) *FileTypeConfig {
	ext := strings.ToLower(filepath.Ext(filename))

	// Special case for Dockerfile
	if strings.HasSuffix(filename, "Dockerfile") {
		if config, ok := p.fileConfigs["Dockerfile"]; ok {
			return &config
		}
	}

	// Check by extension
	for _, config := range p.fileConfigs {
		for _, configExt := range config.Extensions {
			if ext == configExt {
				return &config
			}
		}
	}

	// Check for files without extension (like shell scripts)
	if ext == "" {
		// Try to detect by reading shebang
		const shebangSize = 100
		if content, err := p.readFileHeader(filename, shebangSize); err == nil {
			if strings.HasPrefix(strings.TrimSpace(content), "#!/") {
				if strings.Contains(content, "sh") || strings.Contains(content, "bash") {
					if config, ok := p.fileConfigs[".sh"]; ok {
						return &config
					}
				} else if strings.Contains(content, "python") {
					if config, ok := p.fileConfigs[".py"]; ok {
						return &config
					}
				}
			}
		}
	}

	return nil
}

// readFileHeader reads the first n bytes of a file
func (*LicenseParser) readFileHeader(filename string, size int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, size)
	n, err := file.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return "", err
	}

	return string(buf[:n]), nil
}

// extractLicenseText extracts license text from file header
func (p *LicenseParser) extractLicenseText(header string, config *FileTypeConfig) string {
	// Try each comment style
	for _, style := range config.CommentStyles {
		if text := p.extractWithStyle(header, style); text != "" {
			return text
		}
	}
	return ""
}

// extractWithStyle extracts text using a specific comment style
func (p *LicenseParser) extractWithStyle(header string, style CommentStyle) string {
	if style.BlockStart != "" && style.BlockEnd != "" {
		// Block comment
		startIdx := strings.Index(header, style.BlockStart)
		if startIdx == -1 {
			return ""
		}
		lastStartIdx := startIdx + len(style.BlockStart)

		endIdx := strings.Index(header[lastStartIdx:], style.BlockEnd)
		if endIdx == -1 {
			return ""
		}

		// Extract content between markers
		content := header[lastStartIdx : lastStartIdx+endIdx]
		return p.normalizeText(content)
	} else if style.LinePrefix != "" {
		// Line comments
		scanner := bufio.NewScanner(strings.NewReader(header))
		var lines []string
		inLicense := false

		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Check if line starts with comment prefix
			if strings.HasPrefix(trimmed, style.LinePrefix) {
				content := strings.TrimPrefix(trimmed, style.LinePrefix)
				content = strings.TrimSpace(content)

				// Check if this might be start of license
				if !inLicense && strings.HasPrefix(strings.ToLower(content), "copyright") {
					inLicense = true
				}

				if inLicense {
					lines = append(lines, content)

					// Check if we've reached end of license
					if strings.Contains(content, "limitations under the License") ||
						strings.Contains(content, "See https://github.com/deckhouse/deckhouse") {
						break
					}
				}
			} else if inLicense && trimmed != "" {
				// Non-comment line after license started - stop
				break
			}
		}

		if len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}

	return ""
}

// normalizeText normalizes license text for comparison
func (*LicenseParser) normalizeText(text string) string {
	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")

	// Remove common comment line prefixes
	lines := strings.Split(text, "\n")
	var normalized []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Remove common prefixes like " * " from block comments
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, " *")
		line = strings.TrimPrefix(line, "*")

		normalized = append(normalized, strings.TrimSpace(line))
	}

	return strings.Join(normalized, "\n")
}

// matchLicense checks if text matches a license template
func (p *LicenseParser) matchLicense(text string, license License) (bool, string) {
	// Normalize both texts
	text = p.normalizeText(text)
	template := p.normalizeText(license.Template)

	// Create regex pattern from template
	pattern := buildLicensePattern(template, license.YearPattern)

	// Try to match
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, ""
	}

	match := re.FindStringSubmatch(text)
	if match == nil {
		return false, ""
	}
	if len(match) > 1 {
		return true, match[1]
	}
	return true, "2025" // Default year if not captured
}

func buildLicensePattern(template, yearPattern string) string {
	if yearPattern == "" {
		return regexp.QuoteMeta(template)
	}

	parts := strings.Split(template, "{{YEAR}}")
	var b strings.Builder

	for i, part := range parts {
		left := strings.TrimRight(part, " \t")
		b.WriteString(regexp.QuoteMeta(left))

		if i < len(parts)-1 {
			right := strings.TrimLeft(parts[i+1], " \t")
			parts[i+1] = right

			b.WriteString(`(?:\s*(` + yearPattern + `)\s*|\s*)`)
		}
	}
	return b.String()
}
