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
	"github.com/deckhouse/dmt/pkg/config/global"
)

// RootConfig encapsulates the config data specified in the YAML config file.
type RootConfig struct {
	GlobalSettings global.Global `mapstructure:"global"`
}

type ModuleConfig struct {
	LintersSettings LintersSettings `mapstructure:"linters-settings"`
}

func calculateImpact(v, input *pkg.Level) *pkg.Level {
	if input != nil {
		return input
	}

	if v != nil {
		return v
	}

	lvl := pkg.Error

	return &lvl
}

func NewDefaultRootConfig(dir string) (*RootConfig, error) {
	cfg := &RootConfig{}

	if err := NewLoader(cfg, dir).Load(); err != nil {
		return nil, err
	}

	return cfg, nil
}
