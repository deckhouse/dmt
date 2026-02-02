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
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestOSSRule_OssModuleRule(t *testing.T) {
	tests := []struct {
		name            string
		disable         bool
		createImagesDir bool
		setupFiles      map[string]string
		wantErrors      []string
		wantWarns       []string
	}{
		{
			name:            "rule disabled, no oss.yaml",
			disable:         true,
			createImagesDir: true,
			setupFiles:      map[string]string{},
			wantErrors:      nil,
			wantWarns:       nil,
		},
		{
			name:            "NO images dir, ignore missing oss.yaml",
			createImagesDir: false,
			setupFiles:      map[string]string{},
			wantErrors:      nil,
			wantWarns:       nil,
		},
		{
			name:            "NO images dir, ignore invalid oss.yaml",
			createImagesDir: false,
			setupFiles: map[string]string{
				"oss.yaml": "invalid: yaml: [",
			},
			wantErrors: nil,
			wantWarns:  nil,
		},
		{
			name:            "images dir exists, oss.yaml missing (WARN)",
			createImagesDir: true,
			setupFiles:      map[string]string{},
			wantWarns:       []string{"module has images folder, so it likely should have oss.yaml"},
		},
		{
			name:            "oss.yaml invalid yaml",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": "invalid: yaml: content",
			},
			wantErrors: []string{"error converting YAML to JSON"},
		},
		{
			name:            "oss.yaml empty projects",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": "[]",
			},
			wantErrors: []string{"no projects described"},
		},
		{
			name:            "valid single project",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  license: "Apache License 2.0"
`,
			},
			wantErrors: nil,
		},
		{
			name:            "valid project with logo",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  logo: "https://dexidp.io/img/logos/dex-horizontal-color.png"
  license: "Apache License 2.0"
`,
			},
			wantErrors: nil,
		},
		{
			name:            "project with empty id",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: ""
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  license: "Apache License 2.0"
`,
			},
			wantErrors: []string{"id must not be empty"},
		},
		{
			name:            "project with empty version",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: ""
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  license: "Apache License 2.0"
`,
			},
			wantErrors: []string{"version must not be empty. Please fill in the parameter and configure CI (werf files for module images) to use these setting."},
		},
		{
			name:            "project with invalid semver version",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "invalid-version"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  license: "Apache License 2.0"
`,
			},
			wantWarns: []string{"version must be valid semver"},
		},
		{
			name:            "project with empty name",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: ""
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  license: "Apache License 2.0"
`,
			},
			wantErrors: []string{"name must not be empty"},
		},
		{
			name:            "project with empty description",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: ""
  link: "https://github.com/dexidp/dex"
  license: "Apache License 2.0"
`,
			},
			wantErrors: []string{"description must not be empty"},
		},
		{
			name:            "project with empty link",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: ""
  license: "Apache License 2.0"
`,
			},
			wantErrors: []string{"link must not be empty"},
		},
		{
			name:            "project with invalid link URL",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "not-a-url"
  license: "Apache License 2.0"
`,
			},
			wantErrors: []string{"link URL is malformed"},
		},
		{
			name:            "project with empty license",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  license: ""
`,
			},
			wantErrors: []string{"License must not be empty"},
		},
		{
			name:            "project with invalid logo URL",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  logo: "invalid-logo-url"
  license: "Apache License 2.0"
`,
			},
			wantErrors: []string{"project logo URL is malformed"},
		},
		{
			name:            "multiple projects, one invalid",
			createImagesDir: true,
			setupFiles: map[string]string{
				"oss.yaml": `
- id: "dexidp/dex"
  version: "2.0.0"
  name: "Dex"
  description: "A Federated OpenID Connect Provider with pluggable connectors"
  link: "https://github.com/dexidp/dex"
  license: "Apache License 2.0"
- id: ""
  version: "1.0.0"
  name: "Invalid"
  description: "Invalid project"
  link: "https://example.com"
  license: "MIT"
`,
			},
			wantErrors: []string{"id must not be empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir
			tempDir := t.TempDir()
			// Create images dir if needed
			if tt.createImagesDir {
				if err := os.Mkdir(filepath.Join(tempDir, "images"), 0755); err != nil {
					t.Fatalf("failed to create images dir: %v", err)
				}
			}

			// Setup files
			for filename, content := range tt.setupFiles {
				path := filepath.Join(tempDir, filename)
				if err := os.WriteFile(path, []byte(content), 0644); err != nil { //nolint:gosec // resolve when bump lint
					t.Fatalf("failed to write file %s: %v", filename, err)
				}
			}

			// Create rule
			rule := NewOSSRule(tt.disable)
			errorList := errors.NewLintRuleErrorsList()

			// Run the rule
			rule.OssModuleRule(tempDir, errorList)

			// Check errors
			errs := errorList.GetErrors()
			var errorTexts []string
			var warnTexts []string
			for _, e := range errs {
				switch e.Level {
				case pkg.Error:
					errorTexts = append(errorTexts, e.Text)
				case pkg.Warn:
					warnTexts = append(warnTexts, e.Text)
				}
			}

			checkMessages(t, "error", tt.wantErrors, errorTexts)
			checkMessages(t, "warning", tt.wantWarns, warnTexts)
		})
	}
}

func checkMessages(t *testing.T, msgType string, want []string, got []string) {
	if len(want) == 0 {
		if len(got) > 0 {
			t.Errorf("unexpected %ss: %v", msgType, got)
		}
	} else {
		for _, w := range want {
			found := false
			for _, g := range got {
				if strings.Contains(g, w) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %s containing %q, but not found in %v", msgType, w, got)
			}
		}
	}
}

func Test_parseProjectList(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "empty",
			yaml:      "",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "one",
			wantCount: 1,
			wantErr:   false,
			yaml: `
- id: "1"
  version: "1.0.0"
  name: a
  description: a
  link: https://example.com
  license: Apache 2.0
`,
		},
		{
			name:      "two",
			wantCount: 2,
			wantErr:   false,
			yaml: `
- id: "1"
  version: "1.0.0"
  name: a
  description: a
  link: https://example.com
  license: Apache 2.0
- id: "2"
  version: "1.0.1"
  name: b
  description: b
  link: https://example.com
  license: Apache 2.0
`,
		},
		{
			name:    "invalid yaml",
			yaml:    "invalid: yaml: [",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects, err := parseProjectList([]byte(test.yaml))
			if test.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(projects) != test.wantCount {
				t.Errorf("unexpected project count: got=%d, want=%d", len(projects), test.wantCount)
			}
		})
	}
}
