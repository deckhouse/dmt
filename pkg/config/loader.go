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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/fsutils"
)

type LoaderOptions struct {
	Config string
}

type Loader struct {
	viper *viper.Viper

	cfg any
	dir string
}

func NewLoader(cfg any, dir string) *Loader {
	return &Loader{
		viper: viper.NewWithOptions(),
		cfg:   cfg,
		dir:   dir,
	}
}

func (l *Loader) Load() error {
	err := l.setConfigFile()
	if err != nil {
		return err
	}

	err = l.parseConfig()
	if err != nil {
		return err
	}

	return nil
}

func (l *Loader) setConfigFile() error {
	l.viper.SetConfigName(".dmtlint")

	configSearchPaths := l.getConfigSearchPaths()

	log.Debug("Config search paths", slog.Any("paths", configSearchPaths))

	for _, p := range configSearchPaths {
		l.viper.AddConfigPath(p)
	}

	return nil
}

func (l *Loader) getConfigSearchPaths() []string {
	firstArg := "./..."
	if l.dir != "" {
		firstArg = l.dir
	}

	absPath, err := filepath.Abs(firstArg)
	if err != nil {
		log.Warn("Can't make abs path", slog.String("path", firstArg), log.Err(err))
		absPath = filepath.Clean(firstArg)
	}

	// start from it
	var currentDir string
	if fsutils.IsDir(absPath) {
		currentDir = absPath
	} else {
		currentDir = filepath.Dir(absPath)
	}

	// find all dirs from it up to the root
	searchPaths := []string{}

	for {
		searchPaths = append(searchPaths, currentDir)

		parent := filepath.Dir(currentDir)
		if currentDir == parent || parent == "" {
			break
		}

		currentDir = parent
	}

	// find home directory for global config
	if home, err := homedir.Dir(); err != nil {
		log.Warn("Can't get user's home directory", log.Err(err))
	} else if !slices.Contains(searchPaths, home) {
		searchPaths = append(searchPaths, home)
	}

	return searchPaths
}

func (l *Loader) parseConfig() error {
	if err := l.viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// Load configuration from flags only.
			err = l.viper.Unmarshal(l.cfg, customDecoderHook())
			if err != nil {
				return fmt.Errorf("can't unmarshal config by viper (flags): %w", err)
			}

			return nil
		}

		return fmt.Errorf("can't read viper config: %w", err)
	}

	err := l.setConfigDir()
	if err != nil {
		return err
	}

	// Load configuration from all sources (flags, file).
	if err = l.viper.Unmarshal(l.cfg, customDecoderHook()); err != nil {
		return fmt.Errorf("can't unmarshal config by viper (flags, file): %w", err)
	}

	return validateRbacKeys(l.viper)
}

// rbacKnownKeys lists the keys the rbac blocks accept. viper drops an unknown key without a word,
// and for these blocks silence is expensive: a misspelled per-rule level or exclusion would leave
// a rule at full strength -- or off -- with nobody noticing. Only the rbac blocks are held to
// this; the other linters keep viper's lenient behaviour.
var rbacKnownKeys = map[string]map[string]struct{}{
	"global.linters-settings.rbac":       {"impact": {}, "rules": {}},
	"global.linters-settings.rbac.rules": {"coverage": {}, "sync": {}, "contract": {}},
	"linters-settings.rbac":              {"impact": {}, "exclude-rules": {}},
	"linters-settings.rbac.exclude-rules": {
		"binding-subject": {}, "placement": {}, "wildcards": {}, "coverage": {}, "contract": {}, "sync": {},
	},
}

func validateRbacKeys(v *viper.Viper) error {
	paths := make([]string, 0, len(rbacKnownKeys))
	for path := range rbacKnownKeys {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	for _, path := range paths {
		block, ok := v.Get(path).(map[string]any)
		if !ok {
			continue
		}

		keys := make([]string, 0, len(block))
		for key := range block {
			if _, known := rbacKnownKeys[path][key]; !known {
				keys = append(keys, key)
			}
		}

		if len(keys) > 0 {
			sort.Strings(keys)

			return fmt.Errorf("unknown key(s) %s under %q in %s: the accepted keys are %s",
				strings.Join(keys, ", "), path, v.ConfigFileUsed(), strings.Join(sortedKeysOf(rbacKnownKeys[path]), ", "))
		}
	}

	return nil
}

func sortedKeysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

func (l *Loader) setConfigDir() error {
	usedConfigFile := l.viper.ConfigFileUsed()
	if usedConfigFile == "" {
		return nil
	}

	if usedConfigFile == os.Stdin.Name() {
		usedConfigFile = ""

		log.Info("Reading config file stdin")
	}

	log.Debug("Used config file", slog.String("file", usedConfigFile))

	return nil
}

func customDecoderHook() viper.DecoderConfigOption {
	return viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		// Default hooks (https://github.com/spf13/viper/blob/518241257478c557633ab36e474dfcaeb9a3c623/viper.go#L135-L138).
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),

		// Needed for forbidigo, and output.formats.
		mapstructure.TextUnmarshallerHookFunc(),
	))
}
