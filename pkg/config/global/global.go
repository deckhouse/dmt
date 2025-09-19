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
	Container     ContainerLinterConfig     `mapstructure:"container"`
	Hooks         LinterConfig              `mapstructure:"hooks"`
	Images        ImagesLinterConfig        `mapstructure:"images"`
	License       LinterConfig              `mapstructure:"license"`
	Module        LinterConfig              `mapstructure:"module"`
	NoCyrillic    LinterConfig              `mapstructure:"no-cyrillic"`
	OpenAPI       LinterConfig              `mapstructure:"openapi"`
	Rbac          LinterConfig              `mapstructure:"rbac"`
	Templates     LinterConfig              `mapstructure:"templates"`
	Documentation DocumentationLinterConfig `mapstructure:"documentation"`
}

type LinterConfig struct {
	Impact string `mapstructure:"impact"`
}

type ContainerLinterConfig struct {
	LinterConfig          `mapstructure:",squash"`
	RecommendedLabelsRule RuleConfig `mapstructure:"recommended-labels"`
}

type ImagesLinterConfig struct {
	LinterConfig `mapstructure:",squash"`
	Rules        ImageRules `mapstructure:"rules"`
}

type ImageRules struct {
	DistrolessRule RuleConfig `mapstructure:"distroless"`
	ImageRule      RuleConfig `mapstructure:"image"`
	PatchesRule    RuleConfig `mapstructure:"patches"`
	WerfRule       RuleConfig `mapstructure:"werf"`
}

type RuleConfig struct {
	Impact string `mapstructure:"impact"`
}

type DocumentationLinterConfig struct {
	LinterConfig `mapstructure:",squash"`
	Rules        DocumentationRules `mapstructure:"rules"`
}

type DocumentationRules struct {
	BilingualRule          RuleConfig `mapstructure:"bilingual"`
	ReadmeRule             RuleConfig `mapstructure:"readme"`
	NoCyrillicExcludeRules RuleConfig `mapstructure:"cyrillic-in-english"`
}

func (c LinterConfig) IsWarn() bool {
	return c.Impact == pkg.Warn.String()
}
