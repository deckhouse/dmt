package container

import (
	"testing"

	"github.com/deckhouse/dmt/pkg/config"
)

func Test_shouldSkipModuleContainer(t *testing.T) {
	Cfg = new(config.ContainerSettings)
	Cfg.SkipContainers = []string{
		"okmeter:okagent",
		"d8-control-plane-manager:*.image-holder",
	}
	type args struct {
		md        string
		container string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "true",
			args: args{
				md:        "okmeter",
				container: "okagent",
			}, want: true,
		},
		{
			name: "false",
			args: args{
				md:        "okmeter",
				container: "okagent2",
			}, want: false,
		},
		{
			name: "regexp",
			args: args{
				md:        "d8-control-plane-manager",
				container: "test.image-holder",
			}, want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipModuleContainer(tt.args.md, tt.args.container); got != tt.want {
				t.Errorf("shouldSkipModuleContainer() = %v, want %v", got, tt.want)
			}
		})
	}
}
