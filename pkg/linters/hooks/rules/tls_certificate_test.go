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
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestTLSCertificateRule_CheckFile(t *testing.T) {
	const fixtureDir = "testdata/tls"

	tests := []struct {
		name        string
		fixture     string
		wantErrors  int
		wantSnippet string
	}{
		{
			name:        "invalid requestheader-client usage",
			fixture:     "invalid_usage.go",
			wantErrors:  1,
			wantSnippet: "empty ExtendedKeyUsage",
		},
		{
			name:        "WithGroups on leaf certificate",
			fixture:     "with_groups.go",
			wantErrors:  1,
			wantSnippet: "Subject == Issuer",
		},
		{
			name:       "valid certificate generation",
			fixture:    "valid.go",
			wantErrors: 0,
		},
		{
			name:       "defects without tls_certificate import are skipped",
			fixture:    "no_import.go",
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			rule := NewTLSCertificateRule(false)

			rule.checkFile(fixtureDir, filepath.Join(fixtureDir, tt.fixture), errorList)

			errs := errorList.GetErrors()
			if len(errs) != tt.wantErrors {
				t.Fatalf("expected %d errors, got %d: %+v", tt.wantErrors, len(errs), errs)
			}

			if tt.wantSnippet != "" && !strings.Contains(errs[0].Text, tt.wantSnippet) {
				t.Fatalf("expected error text to contain %q, got %q", tt.wantSnippet, errs[0].Text)
			}
		})
	}
}

func TestTLSCertificateRule_Enabled(t *testing.T) {
	if NewTLSCertificateRule(false).Enabled() != true {
		t.Fatalf("expected rule to be enabled when disable=false")
	}

	if NewTLSCertificateRule(true).Enabled() != false {
		t.Fatalf("expected rule to be disabled when disable=true")
	}
}
