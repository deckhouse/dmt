package license

import (
	"reflect"
	"testing"
)

func Test_getExcludes(t *testing.T) {
	type args struct {
		excludesList []string
		moduleName   string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "exclude .venv folder for managed-pg module",
			args: args{
				excludesList: []string{"managed-pg:/.venv/**/*", "somemodule:/.venv/**/*", "another-mod:/*.txt", "managed-pg:/enabled"},
				moduleName:   "managed-pg",
			},
			want: []string{"/.venv/**/*", "/enabled"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getExcludes(tt.args.excludesList, tt.args.moduleName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getExcludes() = %v, want %v", got, tt.want)
			}
		})
	}
}
