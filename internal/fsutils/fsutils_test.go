/*
Copyright 2024 Flant JSC

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

package fsutils

import (
	"testing"
)

func Test_FSUtils_toRegexp(t *testing.T) {
	type args struct {
		pattern string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test escape slashes",
			args: args{
				pattern: "/home/user/test/project",
			},
			want: "^\\/home\\/user\\/test\\/project$",
		},
		{
			name: "test escape slashes and dots",
			args: args{
				pattern: "/home/user/test/project.txt",
			},
			want: "^\\/home\\/user\\/test\\/project\\.txt$",
		},
		{
			name: "test glob mask **",
			args: args{
				pattern: "/home/user/**/project.txt",
			},
			want: "^\\/home\\/user\\/.*\\/project\\.txt$",
		},
		{
			name: "test glob mask ** and file mask",
			args: args{
				pattern: "/home/user/**/*.txt",
			},
			want: "^\\/home\\/user\\/.*\\/[^/]*\\.txt$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toRegexp(tt.args.pattern); got != tt.want {
				t.Errorf("toRegexp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFSUtils_FileNames_StringMatchMask(t *testing.T) {
	type args struct {
		name    string
		pattern string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "without any mask (exact filenames)",
			args: args{
				name:    "/home/user/test/project.txt",
				pattern: "/home/user/test/project.txt",
			},
			want: true,
		},
		{
			name: "with filename mask and txt extension",
			args: args{
				name:    "/home/user/test/project.txt",
				pattern: "/home/user/test/*.txt",
			},
			want: true,
		},
		{
			name: "with filename mask and log extension",
			args: args{
				name:    "/home/user/test/project.log",
				pattern: "/home/user/test/*.txt",
			},
			want: false,
		},
		{
			name: "with glob mask and exact filename",
			args: args{
				name:    "/home/user/test/project.txt",
				pattern: "/home/user/**/project.txt",
			},
			want: true,
		},
		{
			name: "with glob mask and exact filename which not match with pattern",
			args: args{
				name:    "/home/user/test/newproject.txt",
				pattern: "/home/user/**/project.txt",
			},
			want: false,
		},
		{
			name: "with glob mask and any extension with exact filename",
			args: args{
				name:    "/home/user/test/project.txt",
				pattern: "/home/user/**/project.*",
			},
			want: true,
		},
		{
			name: "with mask without subdirectories",
			args: args{
				name:    "/home/user/test/project.txt",
				pattern: "/home/user/*.txt",
			},
			want: false,
		},
		{
			name: "with glob mask and any file inside directories",
			args: args{
				name:    "/home/user/test/something/other/project.txt",
				pattern: "/home/user/**/*",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringMatchMask(tt.args.name, tt.args.pattern); got != tt.want {
				t.Errorf("StringMatchMask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFSUtils_PlainStrings_StringMatchMask(t *testing.T) {
	type args struct {
		str     string
		pattern string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test module-name:filename with exact name",
			args: args{
				str:     "managed-pg:/enabled",
				pattern: "managed-pg:/enabled",
			},
			want: true,
		},
		{
			name: "test module-name:filename with glob mask",
			args: args{
				str:     "managed-pg:/.venv/lib/python3.13/site-packages/deckhouse/__init__.py",
				pattern: "managed-pg:/.venv/**/*",
			},
			want: true,
		},
		{
			name: "plain string with mask",
			args: args{
				str:     "test-string.delimited.by.dots",
				pattern: "*.by.dots",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringMatchMask(tt.args.str, tt.args.pattern); got != tt.want {
				t.Errorf("StringMatchMask() = %v, want %v", got, tt.want)
			}
		})
	}
}
