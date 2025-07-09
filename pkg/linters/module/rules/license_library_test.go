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
	"testing"
)

func Test_getLicenseType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected LicenseType
	}{
		{
			name:     "CE license for regular file",
			filePath: "internal/module/module.go",
			expected: LicenseTypeCE,
		},
		{
			name:     "CE license for file in regular directory",
			filePath: "pkg/linters/module/rules/license.go",
			expected: LicenseTypeCE,
		},
		{
			name:     "EE license for file in ee directory",
			filePath: "ee/module/rules/license.go",
			expected: LicenseTypeEE,
		},
		{
			name:     "EE license for file in nested ee directory",
			filePath: "internal/ee/module/rules/license.go",
			expected: LicenseTypeEE,
		},
		{
			name:     "EE license for file in EE directory (case insensitive)",
			filePath: "EE/module/rules/license.go",
			expected: LicenseTypeEE,
		},
		{
			name:     "CE license for file in eetools directory",
			filePath: "eetools/module/rules/license.go",
			expected: LicenseTypeCE,
		},
		{
			name:     "CE license for file with ee in middle of path",
			filePath: "internal/feedback/module/rules/license.go",
			expected: LicenseTypeCE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLicenseType(tt.filePath)
			if result != tt.expected {
				t.Errorf("getLicenseType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func Test_ee_license_re(t *testing.T) {
	invalidCases := []struct {
		title   string
		content string
	}{
		{
			title: "No license",
			content: `package main

no license
`,
		},
		{
			title: "CE license instead of EE",
			content: `/*
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

package main
`,
		},
	}

	for _, c := range invalidCases {
		t.Run(c.title, func(t *testing.T) {
			res := EELicenseRe.MatchString(c.content)
			if res {
				t.Errorf("should not detect EE license")
			}
		})
	}

	validCases := []struct {
		title   string
		content string
	}{
		{
			title: "EE license in Go multiline comment",
			content: `/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
*/

package main

import (
  "fmt"
  "os"
)

func main() {
	fmt.Printf("Hello, world!")
    os.Exit(0)
}
`,
		},
		{
			title: "EE license in Go single line comments",
			content: `// Copyright 2025 Flant JSC
// Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;

package main

import (
  "fmt"
  "os"
)

func main() {
	fmt.Printf("Hello, world!")
    os.Exit(0)
}
`,
		},
		{
			title: "EE license in Bash comments",
			content: `#!/bin/bash
# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;

set -Eeo pipefail
`,
		},
		{
			title: "EE license in Lua comments",
			content: `--[[
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
--]]

local a = require "table.nkeys"

print("Hello")
`,
		},
	}

	for _, c := range validCases {
		t.Run(c.title, func(t *testing.T) {
			res := EELicenseRe.MatchString(c.content)
			if !res {
				t.Errorf("should detect EE license")
			}
		})
	}
}

func Test_ee_license_re_debug(t *testing.T) {
	// Test the exact format expected by the regex
	testContent := `/*
Copyright 2025 Flant JSC

Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
*/`

	res := EELicenseRe.MatchString(testContent)
	t.Logf("EELicenseRe pattern: %s", EELicenseRe.String())
	t.Logf("Test content: %q", testContent)
	t.Logf("Match result: %v", res)

	if !res {
		t.Errorf("Expected EE license to match")
	}
}

func Test_ee_license_re_correct_format(t *testing.T) {
	// Test with the exact format that should match the regex
	// The regex expects: Copyright 202[1-9] Flant JSC\n followed by the license text
	testContent := `/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
*/`

	res := EELicenseRe.MatchString(testContent)
	if !res {
		t.Errorf("Expected EE license to match with correct format")
	}
}

func Test_ee_license_re_analysis(t *testing.T) {
	// Let's analyze what the regex expects
	pattern := EELicenseRe.String()
	t.Logf("EE License Regex Pattern: %s", pattern)

	// Test different variations
	testCases := []struct {
		name    string
		content string
	}{
		{
			name: "Exact format with newline after Flant JSC",
			content: `/*
Copyright 2025 Flant JSC

Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
*/`,
		},
		{
			name: "Format without extra newline",
			content: `/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
*/`,
		},
		{
			name: "Format with comment markers",
			content: `/*
Copyright 2025 Flant JSC

Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
*/`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := EELicenseRe.MatchString(tc.content)
			t.Logf("Content: %q", tc.content)
			t.Logf("Match: %v", res)
		})
	}
}

func Test_copyright_re(t *testing.T) {
	in := `package main

no license
`

	res := CELicenseRe.MatchString(in)

	if res {
		t.Errorf("should not detect license")
	}

	validCases := []struct {
		title   string
		content string
	}{
		{
			title: "Bash comment with previous spaces",
			content: `
#!/bin/bash

# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -Eeo pipefail
`,
		},

		{
			title: "Bash comment without previous spaces",
			content: `#!/bin/bash
# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -Eeo pipefail
`,
		},

		{
			title: "Golang multiline comment without previous spaces",
			content: `/*
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

package main

import (
  "fmt"
  "os"
)

func main() {
	fmt.Printf("Hello, world!")
    os.Exit(0)
}
`,
		},

		{
			title: "Golang multiline comment with previous spaces",
			content: `
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
package main

import (
  "fmt"
  "os"
)

func main() {
	fmt.Printf("Hello, world!")
    os.Exit(0)
}
`,
		},

		{
			title: "Golang multiple one line comments without previous spaces",
			content: `// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
  "fmt"
  "os"
)

func main() {
	fmt.Printf("Hello, world!")
    os.Exit(0)
}
`,
		},

		{
			title: "Golang multiple one line comments with previous spaces",
			content: `
// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
  "fmt"
  "os"
)

func main() {
	fmt.Printf("Hello, world!")
    os.Exit(0)
}
`,
		},

		{
			title: "Lua multiple one line comments without previous spaces",
			content: `--[[
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
--]]

local a = require "table.nkeys"

print("Hello")
`,
		},
	}

	for _, c := range validCases {
		t.Run(c.title, func(t *testing.T) {
			res = CELicenseRe.MatchString(c.content)

			if !res {
				t.Errorf("should detect license")
			}
		})
	}
}

func Test_checkFileCopyright_Integration(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()

	// Test CE license file
	ceFile := filepath.Join(tmpDir, "ce_file.go")
	ceContent := `/*
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

package main

func main() {
    fmt.Println("Hello, CE!")
}
`
	if err := os.WriteFile(ceFile, []byte(ceContent), 0644); err != nil {
		t.Fatalf("Failed to create CE test file: %v", err)
	}

	// Test EE license file
	eeDir := filepath.Join(tmpDir, "ee")
	if err := os.MkdirAll(eeDir, 0755); err != nil {
		t.Fatalf("Failed to create EE directory: %v", err)
	}

	eeFile := filepath.Join(eeDir, "ee_file.go")
	eeContent := `/*
Copyright 2025 Flant JSC

Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE;
*/

package main

func main() {
    fmt.Println("Hello, EE!")
}
`
	if err := os.WriteFile(eeFile, []byte(eeContent), 0644); err != nil {
		t.Fatalf("Failed to create EE test file: %v", err)
	}

	// Test file without license
	noLicenseFile := filepath.Join(tmpDir, "no_license.go")
	noLicenseContent := `package main

func main() {
    fmt.Println("No license!")
}
`
	if err := os.WriteFile(noLicenseFile, []byte(noLicenseContent), 0644); err != nil {
		t.Fatalf("Failed to create no license test file: %v", err)
	}

	// Test file with wrong license type
	wrongLicenseFile := filepath.Join(eeDir, "wrong_license.go")
	wrongLicenseContent := `/*
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

package main

func main() {
    fmt.Println("Wrong license in EE directory!")
}
`
	if err := os.WriteFile(wrongLicenseFile, []byte(wrongLicenseContent), 0644); err != nil {
		t.Fatalf("Failed to create wrong license test file: %v", err)
	}

	tests := []struct {
		name        string
		filePath    string
		expectOK    bool
		expectError string
	}{
		{
			name:     "CE file with correct license",
			filePath: ceFile,
			expectOK: true,
		},
		{
			name:     "EE file with correct license",
			filePath: eeFile,
			expectOK: true,
		},
		{
			name:        "File without license",
			filePath:    noLicenseFile,
			expectOK:    false,
			expectError: "no copyright or license information found (expected CE license)",
		},
		{
			name:        "EE file with wrong license type",
			filePath:    wrongLicenseFile,
			expectOK:    false,
			expectError: "file contains Flant references but missing proper EE license header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := checkFileCopyright(tt.filePath)

			if tt.expectOK && !ok {
				t.Errorf("Expected file to be OK, but got error: %v", err)
			}

			if !tt.expectOK && ok {
				t.Errorf("Expected file to have error, but it was OK")
			}

			if !tt.expectOK && err != nil && tt.expectError != "" {
				if err.Error() != tt.expectError {
					t.Errorf("Expected error '%s', but got '%s'", tt.expectError, err.Error())
				}
			}
		})
	}
}
