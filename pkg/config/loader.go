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
	"maps"
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

// rbacKnownKeys lists the keys the rbac blocks accept, by path. viper drops an unknown key without
// a word, and for these blocks silence is expensive: a misspelled per-rule level or exclusion would
// leave a rule at full strength -- or off -- with nobody noticing. Only the rbac blocks are held to
// this; the other linters keep viper's lenient behaviour.
var rbacKnownKeys = map[string]map[string]struct{}{
	"global.linters-settings.rbac":                {"impact": {}, "rules": {}},
	"global.linters-settings.rbac.rules":          {"coverage": {}, "sync": {}, "contract": {}},
	"global.linters-settings.rbac.rules.coverage": {"impact": {}},
	"global.linters-settings.rbac.rules.sync":     {"impact": {}},
	"global.linters-settings.rbac.rules.contract": {"impact": {}},
	"linters-settings.rbac":                       {"impact": {}, "exclude-rules": {}, "rules": {}},
	"linters-settings.rbac.rules":                 {"coverage": {}, "sync": {}, "contract": {}},
	"linters-settings.rbac.rules.coverage":        {"impact": {}},
	"linters-settings.rbac.rules.sync":            {"impact": {}},
	"linters-settings.rbac.rules.contract":        {"impact": {}},
	"linters-settings.rbac.exclude-rules": {
		"binding-subject": {}, "placement": {}, "wildcards": {}, "coverage": {}, "contract": {}, "sync": {},
	},
}

// rbacKnownListKeys lists the exclusion lists whose entries are kind/name pairs; the other lists
// hold plain strings and have no keys to misspell.
var rbacKnownListKeys = map[string]map[string]struct{}{
	"linters-settings.rbac.exclude-rules.placement": {"kind": {}, "name": {}},
	"linters-settings.rbac.exclude-rules.wildcards": {"kind": {}, "name": {}},
	"linters-settings.rbac.exclude-rules.contract":  {"kind": {}, "name": {}},
	"linters-settings.rbac.exclude-rules.sync":      {"kind": {}, "name": {}},
}

// validateRbacKeys reports every unknown key of the rbac blocks in one error, so that a config
// with several misspellings is fixed in one round.
func validateRbacKeys(v *viper.Viper) error {
	var problems []string

	for _, path := range slices.Sorted(maps.Keys(rbacKnownKeys)) {
		block, ok := v.Get(path).(map[string]any)
		if !ok {
			continue
		}

		problems = append(problems, unknownKeys(block, rbacKnownKeys[path], path)...)

		// A level that is not one of the known ones is read as error: "ignore" for "ignored" would
		// raise a rule instead of silencing it.
		if impact, set := block["impact"]; set {
			if s, isString := impact.(string); !isString || !knownLevels[s] {
				problems = append(problems, fmt.Sprintf("%s.impact is %v: the levels are ignored, warn, error, critical", path, impact))
			}
		}
	}

	for _, path := range slices.Sorted(maps.Keys(rbacKnownListKeys)) {
		list, ok := v.Get(path).([]any)
		if !ok {
			continue
		}

		for i, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				problems = append(problems, fmt.Sprintf("entry %d under %q is not a kind/name pair", i, path))
				continue
			}

			problems = append(problems, unknownKeys(entry, rbacKnownListKeys[path], fmt.Sprintf("%s[%d]", path, i))...)
		}
	}

	if len(problems) == 0 {
		return nil
	}

	return fmt.Errorf("%s in %s", strings.Join(problems, "; "), v.ConfigFileUsed())
}

// knownLevels are the impact values pkg.ParseStringToLevel knows.
var knownLevels = map[string]bool{"ignored": true, "warn": true, "error": true, "critical": true}

// unknownKeys names the keys of block that known does not list, with the accepted ones.
func unknownKeys(block map[string]any, known map[string]struct{}, path string) []string {
	var unknown []string

	for key := range block {
		if _, ok := known[key]; !ok {
			unknown = append(unknown, key)
		}
	}

	if len(unknown) == 0 {
		return nil
	}

	sort.Strings(unknown)

	return []string{fmt.Sprintf("unknown key(s) %s under %q: the accepted keys are %s",
		strings.Join(unknown, ", "), path, strings.Join(slices.Sorted(maps.Keys(known)), ", "))}
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
