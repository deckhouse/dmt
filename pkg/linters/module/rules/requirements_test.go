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

package rules

import (
	"testing"

	"github.com/deckhouse/dmt/pkg/errors"
)

func Test_checkStage(t *testing.T) {
	type args struct {
		module   *DeckhouseModule
		expected bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "module is empty",
			args: args{
				module:   nil,
				expected: false,
			},
		},
		{
			name: "stage is not empty",
			args: args{
				module: &DeckhouseModule{
					Stage:        "1.68.0",
					Requirements: &ModuleRequirements{},
				},
				expected: true,
			},
		},
		{
			name: "requirements is empty",
			args: args{
				module: &DeckhouseModule{
					Stage:        "1.68.0",
					Requirements: nil,
				},
				expected: true,
			},
		},
		{
			name: "requirements is not empty",
			args: args{
				module: &DeckhouseModule{
					Stage: "1.68.0",
					Requirements: &ModuleRequirements{
						ModulePlatformRequirements: ModulePlatformRequirements{
							Deckhouse: ">= 1.68",
						},
					},
				},
				expected: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			checkStage(tt.args.module, errorList)
			if errorList.ContainsErrors() != tt.args.expected {
				t.Errorf("errorList: %v", errorList)
			}
		})
	}
}
