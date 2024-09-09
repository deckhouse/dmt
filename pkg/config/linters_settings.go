package config

import (
	"errors"
	"fmt"
)

// var defaultLintersSettings = LintersSettings{}

type LintersSettings struct {
	Custom map[string]CustomLinterSettings
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
