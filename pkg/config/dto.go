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

package config

import (
	"github.com/deckhouse/dmt/pkg"
)

type UserRuleSettings struct {
	Impact *pkg.Level `mapstructure:"impact"`
}

type UserLinterSettings struct {
	Impact        *pkg.Level                  `mapstructure:"impact"`
	RulesSettings map[string]UserRuleSettings `mapstructure:"rules-settings"`
	ExcludeRules  []string                    `mapstructure:"exclude-rules"`
}

type UserLintersSettings struct {
	Container  UserLinterSettings `mapstructure:"container"`
	Hooks      UserLinterSettings `mapstructure:"hooks"`
	Images     UserLinterSettings `mapstructure:"images"`
	License    UserLinterSettings `mapstructure:"license"`
	Module     UserLinterSettings `mapstructure:"module"`
	NoCyrillic UserLinterSettings `mapstructure:"no-cyrillic"`
	OpenAPI    UserLinterSettings `mapstructure:"openapi"`
	Rbac       UserLinterSettings `mapstructure:"rbac"`
	Templates  UserLinterSettings `mapstructure:"templates"`
}

type UserRootConfig struct {
	LintersSettings UserLintersSettings `mapstructure:"linters-settings"`
}
