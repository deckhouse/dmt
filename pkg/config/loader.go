package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	"github.com/creasty/defaults"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/pkg"
)

type LoaderOptions struct {
	Config string
}

type Loader struct {
	viper *viper.Viper

	cfg  any
	args []string
}

func NewLoader(cfg any, dirs ...string) *Loader {
	return &Loader{
		viper: viper.NewWithOptions(),
		cfg:   cfg,
		args:  dirs,
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

	logger.InfoF("Config search paths: %s", configSearchPaths)

	for _, p := range configSearchPaths {
		l.viper.AddConfigPath(p)
	}

	return nil
}

func (l *Loader) getConfigSearchPaths() []string {
	firstArg := "./..."
	if len(l.args) > 0 {
		firstArg = l.args[0]
	}

	absPath, err := filepath.Abs(firstArg)
	if err != nil {
		logger.WarnF("Can't make abs path for %q: %s", firstArg, err)
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
		logger.WarnF("Can't get user's home directory: %v", err)
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

	if err = defaults.Set(l.cfg); err != nil {
		return fmt.Errorf("set defaults: %w", err)
	}

	// Load configuration from all sources (flags, file).
	if err = l.viper.Unmarshal(l.cfg, customDecoderHook()); err != nil {
		return fmt.Errorf("can't unmarshal config by viper (flags, file): %w", err)
	}

	return nil
}

func (l *Loader) setConfigDir() error {
	usedConfigFile := l.viper.ConfigFileUsed()
	if usedConfigFile == "" {
		return nil
	}

	if usedConfigFile == os.Stdin.Name() {
		usedConfigFile = ""
		logger.InfoF("Reading config file stdin")
	}

	logger.InfoF("Used config file %s", usedConfigFile)

	return nil
}

func customDecoderHook() viper.DecoderConfigOption {
	return viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		StringToLevelHookFunc(),
		// Default hooks (https://github.com/spf13/viper/blob/518241257478c557633ab36e474dfcaeb9a3c623/viper.go#L135-L138).
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),

		// Needed for forbidigo, and output.formats.
		mapstructure.TextUnmarshallerHookFunc(),
	))
}

func StringToLevelHookFunc() mapstructure.DecodeHookFuncType {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String || f.Kind() == reflect.Pointer {
			return data, nil
		}

		if !strings.Contains(t.String(), "Level") {
			return data, nil
		}

		return pkg.ParseStringToLevel(data.(string)), nil
	}
}
