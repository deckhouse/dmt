package module

import (
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
)

func Test_parseProperties(t *testing.T) {
	type args struct {
		tempNode *spec.Schema
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]any
		wantErr bool
	}{
		{
			name: "test foo bar",
			args: args{
				tempNode: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						ID:       "test",
						Type:     spec.StringOrArray{"object"},
						Default:  nil,
						Required: []string{"foo"},
						Properties: map[string]spec.Schema{
							"foo": {
								SchemaProps: spec.SchemaProps{
									Type:    spec.StringOrArray{"string"},
									Default: "text",
								},
							},
							"bar": {
								SchemaProps: spec.SchemaProps{
									Type:    spec.StringOrArray{"string"},
									Default: "text",
								},
							},
							"empty": {
								SchemaProps: spec.SchemaProps{
									Type: spec.StringOrArray{"string"},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"foo": "text",
				"bar": "text",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProperties(tt.args.tempNode)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseProperties() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
