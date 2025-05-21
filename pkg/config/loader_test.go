package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Field1 string `mapstructure:"field1"`
	Field2 int    `mapstructure:"field2"`
}

func TestLoader_Load_ConfigFileNotFound(t *testing.T) {
	cfg := &testConfig{}
	l := NewLoader(cfg, "")
	l.viper = viper.New()
	l.viper.SetConfigName("nonexistent")
	l.viper.AddConfigPath(os.TempDir())

	err := l.Load()
	require.NoError(t, err)
}

func TestLoader_Load_WithConfigFile(t *testing.T) {
	cfg := &testConfig{}
	l := NewLoader(cfg, "")
	l.viper = viper.New()
	l.viper.SetConfigType("yaml")
	l.viper.SetConfigName("testconfig")
	dir := t.TempDir()
	l.viper.AddConfigPath(dir)

	fileContent := []byte("field1: value1\nfield2: 42\n")
	filePath := dir + "/testconfig.yaml"
	err := os.WriteFile(filePath, fileContent, 0600)
	require.NoError(t, err, "failed to write config file")

	// Debug: check file exists and print content
	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "failed to read config file")
	t.Logf("config file content: %s", string(data))

	// Direct viper read/unmarshal for debug
	err = l.viper.ReadInConfig()
	require.NoError(t, err, "viper.ReadInConfig failed")
	err = l.viper.Unmarshal(cfg)
	require.NoError(t, err, "viper.Unmarshal failed")
	t.Logf("direct viper cfg: %+v", cfg)

	// Now test Loader
	cfg2 := &testConfig{}
	l2 := NewLoader(cfg2, "")
	l2.viper = l.viper
	err = l2.Load()
	require.NoError(t, err)
	t.Logf("Loader cfg: %+v", cfg2)
	assert.Equal(t, "value1", cfg2.Field1)
	assert.Equal(t, 42, cfg2.Field2)
}

func TestLoader_setConfigDir_Stdin(t *testing.T) {
	cfg := &testConfig{}
	l := NewLoader(cfg, "")
	l.viper = viper.New()
	l.viper.SetConfigFile(os.Stdin.Name())
	err := l.setConfigDir()
	require.NoError(t, err)
}
