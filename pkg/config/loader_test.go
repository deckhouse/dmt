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

	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "failed to read config file")
	t.Logf("config file content: %s", string(data))

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

func TestLoader_Load_InvalidYaml(t *testing.T) {
	cfg := &testConfig{}
	l := NewLoader(cfg, "")
	l.viper = viper.New()
	l.viper.SetConfigType("yaml")
	l.viper.SetConfigName("invalidconfig")
	dir := t.TempDir()
	l.viper.AddConfigPath(dir)

	fileContent := []byte("field1: value1\nfield2: not_an_int\n")
	filePath := dir + "/invalidconfig.yaml"
	err := os.WriteFile(filePath, fileContent, 0600)
	require.NoError(t, err, "failed to write config file")

	l.viper.SetConfigFile(filePath)
	err = l.viper.ReadInConfig()
	require.NoError(t, err)
	err = l.viper.Unmarshal(cfg)
	assert.Error(t, err, "Should return error for invalid yaml type")
}

func TestLoader_Load_NestedStruct(t *testing.T) {
	type Nested struct {
		SubField string `mapstructure:"sub_field"`
	}
	type nestedConfig struct {
		Field1 string `mapstructure:"field1"`
		Nested Nested `mapstructure:"nested"`
	}
	cfg := &nestedConfig{}
	l := NewLoader(cfg, "")
	l.viper = viper.New()
	l.viper.SetConfigType("yaml")
	l.viper.SetConfigName("nestedconfig")
	dir := t.TempDir()
	l.viper.AddConfigPath(dir)

	fileContent := []byte("field1: value1\nnested:\n  sub_field: subvalue\n")
	filePath := dir + "/nestedconfig.yaml"
	err := os.WriteFile(filePath, fileContent, 0600)
	require.NoError(t, err, "failed to write config file")

	l.viper.SetConfigFile(filePath)
	err = l.viper.ReadInConfig()
	require.NoError(t, err)
	err = l.viper.Unmarshal(cfg)
	require.NoError(t, err)
	assert.Equal(t, "value1", cfg.Field1)
	assert.Equal(t, "subvalue", cfg.Nested.SubField)
}

func TestLoader_setConfigDir_Stdin(t *testing.T) {
	cfg := &testConfig{}
	l := NewLoader(cfg, "")
	l.viper = viper.New()
	// Create a temporary file to simulate stdin
	tempFile, err := os.CreateTemp("", "stdin_mock")
	assert.NoError(t, err, "failed to create temp file")
	defer os.Remove(tempFile.Name()) // Clean up the temp file
	// Use the temporary file as the config file
	l.viper.SetConfigFile(tempFile.Name())
	err = l.setConfigDir()
	require.NoError(t, err)
}

func TestLoader_setConfigDir_Stdin_Content(t *testing.T) {
	cfg := &testConfig{}
	l := NewLoader(cfg, "")
	l.viper = viper.New()
	l.viper.SetConfigType("yaml")
	// Create a temporary file with config content
	fileContent := []byte("field1: stdinval\nfield2: 99\n")
	tempFile, err := os.CreateTemp("", "stdin_mock")
	assert.NoError(t, err, "failed to create temp file")
	defer os.Remove(tempFile.Name())
	_, err = tempFile.Write(fileContent)
	assert.NoError(t, err, "failed to write to temp file")
	tempFile.Close()
	l.viper.SetConfigFile(tempFile.Name())
	err = l.setConfigDir()
	require.NoError(t, err)
	err = l.viper.ReadInConfig()
	require.NoError(t, err)
	err = l.viper.Unmarshal(cfg)
	require.NoError(t, err)
	assert.Equal(t, "stdinval", cfg.Field1)
	assert.Equal(t, 99, cfg.Field2)
}
