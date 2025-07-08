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

package reggen

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	type args struct {
		regex string
		limit int
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
