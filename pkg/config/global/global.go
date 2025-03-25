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

package global

import "github.com/deckhouse/dmt/pkg"

type Global struct {
	Linters Linters `mapstructure:"linters-settings"`
}

type Linters struct {
	Container  LinterConfig `mapstructure:"container"`
	Hooks      LinterConfig `mapstructure:"hooks"`
	Images     LinterConfig `mapstructure:"images"`
	License    LinterConfig `mapstructure:"license"`
	Module     LinterConfig `mapstructure:"module"`
	NoCyrillic LinterConfig `mapstructure:"no-cyrillic"`
	OpenAPI    LinterConfig `mapstructure:"openapi"`
	Rbac       LinterConfig `mapstructure:"rbac"`
	Templates  LinterConfig `mapstructure:"templates"`
}

type LinterConfig struct {
	Impact *pkg.Level `mapstructure:"impact"`
}

func (c LinterConfig) IsWarn() bool {
	return c.Impact != nil && *c.Impact == pkg.Warn
}
