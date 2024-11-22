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
			want: "^\\/home\\/user\\/.*project\\.txt$",
		},
		{
			name: "test glob mask ** and file mask",
			args: args{
				pattern: "/home/user/**/*.txt",
			},
			want: "^\\/home\\/user\\/.*[^/]*\\.txt$",
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

func TestFSUtils_FileNameMatchMask(t *testing.T) {
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
			name: "with glob mask and any extension with exact filename",
			args: args{
				name:    "/home/user/test/project.txt",
				pattern: "/home/user/**/project.*",
			},
			want: true,
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
			if got := FileNameMatchMask(tt.args.name, tt.args.pattern); got != tt.want {
				t.Errorf("FileNameMatchMask() = %v, want %v", got, tt.want)
			}
		})
	}
}
