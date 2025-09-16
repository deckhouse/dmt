package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type DTOLoader struct {
	viper *viper.Viper
	dir   string
}

func NewDTOLoader(dir string) *DTOLoader {
	return &DTOLoader{
		viper: viper.NewWithOptions(),
		dir:   dir,
	}
}

func (l *DTOLoader) LoadUserConfig() (*UserRootConfigDTO, error) {
	l.viper.SetConfigName(".dmtlint")
	l.viper.SetConfigType("yaml")
	l.viper.AddConfigPath(l.dir)

	if err := l.viper.ReadInConfig(); err != nil {
		return &UserRootConfigDTO{}, nil
	}

	var userDTO UserRootConfigDTO
	if err := l.viper.Unmarshal(&userDTO, customDecoderHook()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user config: %w", err)
	}

	return &userDTO, nil
}

func (l *DTOLoader) LoadGlobalConfig() (*GlobalRootConfigDTO, error) {
	l.viper.SetConfigName(".dmtlint")
	l.viper.SetConfigType("yaml")
	l.viper.AddConfigPath(".")

	if err := l.viper.ReadInConfig(); err != nil {
		return &GlobalRootConfigDTO{}, nil
	}

	var globalDTO GlobalRootConfigDTO
	if err := l.viper.Unmarshal(&globalDTO, customDecoderHook()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal global config: %w", err)
	}

	return &globalDTO, nil
}
