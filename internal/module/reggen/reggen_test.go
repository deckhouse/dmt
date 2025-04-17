package reggen

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	type args struct {
		regex string
		limit int
		len   int
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "valid regex",
			args: args{
				regex: "a{3}",
			},
			want:    "aaa",
			wantErr: false,
		},
		{
			name: "valid regex with limit and length",
			args: args{
				regex: "a{1,4}",
				limit: 2,
			},
			want: "aa",
		},
		{
			name: "valid regex with limit and length",
			args: args{
				regex: "a{1,2}",
				limit: 4,
			},
			want: "aa",
		},
		{
			name: "valid regex with plus",
			args: args{
				regex: "^a+$",
				limit: 8,
			},
			want: "aaaaaaaa",
		},
		{
			name: "valid regex with stars",
			args: args{
				regex: "^a*",
				limit: 8,
			},
			want: "aaaaaaaa",
		},
		{
			name: "valid regex with question",
			args: args{
				regex: "^a?$",
				limit: 8,
			},
			want: "a",
		},
		{
			name: "valid regex with question and group",
			args: args{
				regex: "^(azz)?$",
				limit: 8,
			},
			want: "azz",
		},
		{
			name: "invalid regex",
			args: args{
				regex: "[a-z",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Generate(tt.args.regex, tt.args.limit)
			t.Logf("Generate() got = %v, want %v", got, tt.want)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Generate() got = %v, want %v", got, tt.want)
			}
		})
	}
}
