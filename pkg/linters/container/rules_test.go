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

package container

import (
	"testing"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

func Test_shouldSkipModuleContainer(t *testing.T) {
	cfg := &config.ModuleConfig{
		LintersSettings: config.LintersSettings{
			Container: config.ContainerSettings{
				SkipContainers: []string{
					"okmeter:okagent",
					"d8-control-plane-manager:*image-holder",
				},
			},
		},
	}

	linter := New(cfg, errors.NewLintRuleErrorsList())

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
			if got := linter.shouldSkipModuleContainer(tt.args.md, tt.args.container); got != tt.want {
				t.Errorf("shouldSkipModuleContainer() = %v, want %v", got, tt.want)
			}
		})
	}
}
