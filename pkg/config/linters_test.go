package config

import (
	"testing"
)

func TestLinters_validateDisabledAndEnabledAtOneMoment(t *testing.T) {
	type fields struct {
		Enable     []string
		Disable    []string
		EnableAll  bool
		DisableAll bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "enabled one",
			fields: fields{
				Enable:     []string{"test"},
				Disable:    nil,
				EnableAll:  false,
				DisableAll: false,
			},
			wantErr: false,
		},
		{
			name: "enabled / disabled",
			fields: fields{
				Enable:     []string{"test"},
				Disable:    []string{"test"},
				EnableAll:  false,
				DisableAll: false,
			},
			wantErr: true,
		},
		{
			name: "disabled one",
			fields: fields{
				Enable:     nil,
				Disable:    []string{"test"},
				EnableAll:  false,
				DisableAll: false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Linters{
				Enable:     tt.fields.Enable,
				Disable:    tt.fields.Disable,
				EnableAll:  tt.fields.EnableAll,
				DisableAll: tt.fields.DisableAll,
			}
			if err := l.validateDisabledAndEnabledAtOneMoment(); (err != nil) != tt.wantErr {
				t.Errorf("validateDisabledAndEnabledAtOneMoment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
