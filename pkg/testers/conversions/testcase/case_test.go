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

package testcase

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	tmpDir := t.TempDir()

	convDir := filepath.Join(tmpDir, "openapi/conversions")
	err := os.MkdirAll(convDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	v2yaml := `
version: 2
conversions:
  - del(.auth.password) | if .auth == {} then del(.auth) end
description:
  ru: "test"
  en: "test"
`
	err = os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(v2yaml), 0644)
	if err != nil {
		t.Fatal(err)
	}

	testcasesYAML := `
testcases:
  - name: "should delete auth.password on 1 to 2"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
        allowedUserGroups:
          - group1
    expected: |
      auth:
        allowedUserGroups:
          - group1
`
	err = os.WriteFile(filepath.Join(convDir, "testcases.yaml"), []byte(testcasesYAML), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = New().Run(tmpDir)
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
}
