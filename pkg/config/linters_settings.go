package config

import (
	"errors"
	"fmt"
)

type LintersSettings struct {
	OpenAPI    OpenAPISettings                 `mapstructure:"openapi"`
	NoCyrillic NoCyrillicSettings              `mapstructure:"nocyrillic"`
	Copyright  CopyrightSettings               `mapstructure:"copyright"`
	Probes     ProbesSettings                  `mapstructure:"probes"`
	Custom     map[string]CustomLinterSettings `mapstructure:"custom"`
}

func (s *LintersSettings) Validate() error {
	for name, settings := range s.Custom {
		if err := settings.Validate(); err != nil {
			return fmt.Errorf("custom linter %q: %w", name, err)
		}
	}

	return nil
}

// CustomLinterSettings encapsulates the meta-data of a private linter.
type CustomLinterSettings struct {
	// Type plugin type.
	// It can be `goplugin` or `module`.
	Type string `mapstructure:"type"`

	// Path to a plugin *.so file that implements the private linter.
	// Only for Go plugin system.
	Path string

	// Description describes the purpose of the private linter.
	Description string
	// OriginalURL The URL containing the source code for the private linter.
	OriginalURL string `mapstructure:"original-url"`

	// Settings plugin settings only work with linterdb.PluginConstructor symbol.
	Settings any
}

func (s *CustomLinterSettings) Validate() error {
	if s.Type == "module" {
		if s.Path != "" {
			return errors.New("path not supported with module type")
		}

		return nil
	}

	if s.Path == "" {
		return errors.New("path is required")
	}

	return nil
}

type OpenAPISettings struct {
	// EnumFileExcludes contains map with key string contained module name and file path separated by :
	EnumFileExcludes       map[string][]string `mapstructure:"enum-file-excludes"`
	HAAbsoluteKeysExcludes map[string]string   `mapstructure:"ha-absolute-keys-excludes"`
	KeyBannedNames         []string            `mapstructure:"key-banned-names"`
}

type NoCyrillicSettings struct {
	NoCyrillicFileExcludes map[string]struct{} `mapstructure:"no-cyrillic-file-excludes"`
	FileExtensions         []string            `mapstructure:"file-extensions"`
	SkipDocRe              string              `mapstructure:"skip-doc-re"`
	SkipI18NRe             string              `mapstructure:"skip-i18n-re"`
	SkipSelfRe             string              `mapstructure:"skip-self-re"`
}

type CopyrightSettings struct {
	CopyrightExcludes map[string]struct{} `mapstructure:"copyright-excludes"`
}

type ProbesSettings struct {
	ProbesExcludes map[string]map[string]struct{} `mapstructure:"probes-excludes"`
}
