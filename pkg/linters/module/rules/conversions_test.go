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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestNewConversionsRule(t *testing.T) {
	tests := []struct {
		name     string
		disable  bool
		expected bool
	}{
		{
			name:     "enabled rule",
			disable:  false,
			expected: true,
		},
		{
			name:     "disabled rule",
			disable:  true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewConversionsRule(tt.disable)
			assert.Equal(t, ConversionsRuleName, rule.GetName())
			assert.Equal(t, tt.expected, rule.Enabled())
		})
	}
}

func TestConversionsRule_CheckConversions(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(dir string) error
		expectedErrors []string
	}{
		{
			name: "no config-values.yaml",
			setup: func(dir string) error {
				return nil
			},
			expectedErrors: []string{},
		},
		{
			name: "config-values with x-config-version 0 and no conversions folder",
			setup: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, "openapi"), 0755)
			},
			expectedErrors: []string{},
		},
		{
			name: "x-config-version set but no conversions folder",
			setup: func(dir string) error {
				openapiDir := filepath.Join(dir, "openapi")

				if err := os.MkdirAll(openapiDir, 0755); err != nil {
					return err
				}

				return os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte("x-config-version: 2"), 0644)
			},
			expectedErrors: []string{
				"Conversions folder is not exist",
			},
		},
		{
			name: "conversions exist but x-config-version is 0",
			setup: func(dir string) error {
				openapiDir := filepath.Join(dir, "openapi")
				convDir := filepath.Join(openapiDir, "conversions")

				if err := os.MkdirAll(convDir, 0755); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte("x-config-version: 0"), 0644); err != nil {
					return err
				}

				return os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(`version: 2
conversions:
  - del(.auth.password)
description:
  en: "v2"
  ru: "v2 ru"`), 0644)
			},
			expectedErrors: []string{
				"x-config-version is not set in config-values.yaml, but conversions exist",
			},
		},
		{
			name: "x-config-version does not match latest conversion version",
			setup: func(dir string) error {
				openapiDir := filepath.Join(dir, "openapi")
				convDir := filepath.Join(openapiDir, "conversions")

				if err := os.MkdirAll(convDir, 0755); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte("x-config-version: 5"), 0644); err != nil {
					return err
				}

				return os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(`version: 2
conversions:
  - del(.auth.password)
description:
  en: "v2"
  ru: "v2 ru"`), 0644)
			},
			expectedErrors: []string{
				"x-config-version (5) does not match latest conversion version (2)",
			},
		},
		{
			name: "valid conversions with matching x-config-version",
			setup: func(dir string) error {
				openapiDir := filepath.Join(dir, "openapi")
				convDir := filepath.Join(openapiDir, "conversions")

				if err := os.MkdirAll(convDir, 0755); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte("x-config-version: 2"), 0644); err != nil {
					return err
				}

				return os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(`version: 2
conversions:
  - del(.auth.password)
description:
  en: "v2"
  ru: "v2 ru"`), 0644)
			},
			expectedErrors: []string{},
		},
		{
			name: "conversions not starting from version 2",
			setup: func(dir string) error {
				openapiDir := filepath.Join(dir, "openapi")
				convDir := filepath.Join(openapiDir, "conversions")

				if err := os.MkdirAll(convDir, 0755); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte("x-config-version: 3"), 0644); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(convDir, "v3.yaml"), []byte(`version: 3
conversions:
  - del(.auth.password)
description:
  en: "v3"
  ru: "v3 ru"`), 0644); err != nil {
					return err
				}

				return nil
			},
			expectedErrors: []string{
				"You need to start with version number: 2",
			},
		},
		{
			name: "non-sequential conversion versions",
			setup: func(dir string) error {
				openapiDir := filepath.Join(dir, "openapi")
				convDir := filepath.Join(openapiDir, "conversions")

				if err := os.MkdirAll(convDir, 0755); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte("x-config-version: 4"), 0644); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(`version: 2
conversions:
  - del(.auth.password)
description:
  en: "v2"
  ru: "v2 ru"`), 0644); err != nil {
					return err
				}

				return os.WriteFile(filepath.Join(convDir, "v4.yaml"), []byte(`version: 4
conversions:
  - del(.auth)
description:
  en: "v4"
  ru: "v4 ru"`), 0644)
			},
			expectedErrors: []string{
				"No sequential versions between 4 and 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			err := tt.setup(tempDir)
			require.NoError(t, err)

			rule := NewConversionsRule(false)
			errorList := errors.NewLintRuleErrorsList()

			rule.CheckConversions(tempDir, errorList)

			errs := errorList.GetErrors()
			assert.Len(t, errs, len(tt.expectedErrors), "Expected %d errors, got %d", len(tt.expectedErrors), len(errs))

			for i, expectedError := range tt.expectedErrors {
				if i < len(errs) {
					assert.Contains(t, errs[i].Text, expectedError)
				}
			}
		})
	}
}
