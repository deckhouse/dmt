package werf

import (
	"path/filepath"
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
