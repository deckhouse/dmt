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

package werf

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGetWerfConfig(t *testing.T) {
	// Setup temporary directory for testing

	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{
			name:    "Valid werf.yaml file",
			dir:     "testdata/modules/021-cni-cilium",
			wantErr: false,
		},
		{
			name:    "No werf.yaml file",
			dir:     filepath.Join("testdata", "nonexistent"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetWerfConfig(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWerfConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetWerfConfigWithFilesExists(t *testing.T) {
	tests := []struct {
		name               string
		dir                string
		expectedContains   []string
		unexpectedContains []string
		wantErr            bool
	}{
		{
			name:               "Test werf.yaml with Files.Exists",
			dir:                "testdata",
			expectedContains:   []string{"module-cni-cilium: exists", "nonexistent: missing"},
			unexpectedContains: []string{"module-cni-cilium: missing", "nonexistent: exists"},
			wantErr:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetWerfConfig(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWerfConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for _, expected := range tt.expectedContains {
					if !strings.Contains(config, expected) {
						t.Errorf("GetWerfConfig() config = %v, expected to contain %v", config, expected)
					}
				}
				for _, unexpected := range tt.unexpectedContains {
					if strings.Contains(config, unexpected) {
						t.Errorf("GetWerfConfig() config = %v, should not contain %v", config, unexpected)
					}
				}
			}
		})
	}
}
