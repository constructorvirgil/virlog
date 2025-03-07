package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试从JSON文件加载配置
func TestLoadFromJSONFile(t *testing.T) {
	// 创建临时JSON配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	jsonContent := `{
		"level": "debug",
		"format": "console",
		"output": "stdout",
		"development": true,
		"enable_caller": true,
		"enable_stacktrace": false,
		"default_fields": {
			"app": "test-app",
			"env": "testing"
		},
		"file_config": {
			"filename": "test.log",
			"max_size": 10,
			"max_backups": 5,
			"max_age": 30,
			"compress": true
		}
	}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// 加载配置
	config, err := LoadFromFile(configPath)
	require.NoError(t, err)

	// 验证配置
	assert.Equal(t, "debug", config.Level)
	assert.Equal(t, "console", config.Format)
	assert.Equal(t, "stdout", config.Output)
	assert.True(t, config.Development)
	assert.True(t, config.EnableCaller)
	assert.False(t, config.EnableStacktrace)

	// 验证默认字段
	assert.Equal(t, "test-app", config.DefaultFields["app"])
	assert.Equal(t, "testing", config.DefaultFields["env"])

	// 验证文件配置
	assert.Equal(t, "test.log", config.FileConfig.Filename)
	assert.Equal(t, 10, config.FileConfig.MaxSize)
	assert.Equal(t, 5, config.FileConfig.MaxBackups)
	assert.Equal(t, 30, config.FileConfig.MaxAge)
	assert.True(t, config.FileConfig.Compress)
}

// 测试从YAML文件加载配置
func TestLoadFromYAMLFile(t *testing.T) {
	// 创建临时YAML配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	yamlContent := `
level: warn
format: json
output: file
development: false
enable_caller: false
enable_stacktrace: true
enable_sampling: true
default_fields:
  app: yaml-app
  env: production
file_config:
  filename: app.log
  max_size: 20
  max_backups: 3
  max_age: 14
  compress: false
`

	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// 加载配置
	config, err := LoadFromFile(configPath)
	require.NoError(t, err)

	// 验证配置
	assert.Equal(t, "warn", config.Level)
	assert.Equal(t, "json", config.Format)
	assert.Equal(t, "file", config.Output)
	assert.False(t, config.Development)
	assert.False(t, config.EnableCaller)
	assert.True(t, config.EnableStacktrace)
	assert.True(t, config.EnableSampling)

	// 验证默认字段
	assert.Equal(t, "yaml-app", config.DefaultFields["app"])
	assert.Equal(t, "production", config.DefaultFields["env"])

	// 验证文件配置
	assert.Equal(t, "app.log", config.FileConfig.Filename)
	assert.Equal(t, 20, config.FileConfig.MaxSize)
	assert.Equal(t, 3, config.FileConfig.MaxBackups)
	assert.Equal(t, 14, config.FileConfig.MaxAge)
	assert.False(t, config.FileConfig.Compress)
}

// 测试保存配置到文件
func TestSaveToFile(t *testing.T) {
	// 创建配置对象
	config := &Config{
		Level:            "error",
		Format:           "json",
		Output:           "file",
		Development:      false,
		EnableCaller:     true,
		EnableStacktrace: true,
		DefaultFields: map[string]interface{}{
			"service": "save-test",
		},
		FileConfig: &FileConfig{
			Filename:   "save_test.log",
			MaxSize:    5,
			MaxBackups: 2,
			MaxAge:     7,
			Compress:   true,
		},
	}

	// 测试保存为JSON
	tempDir := t.TempDir()
	jsonPath := filepath.Join(tempDir, "saved_config.json")

	err := SaveToFile(config, jsonPath)
	require.NoError(t, err)

	// 重新加载并验证
	loadedConfig, err := LoadFromFile(jsonPath)
	require.NoError(t, err)

	assert.Equal(t, config.Level, loadedConfig.Level)
	assert.Equal(t, config.Format, loadedConfig.Format)
	assert.Equal(t, config.Output, loadedConfig.Output)
	assert.Equal(t, "save-test", loadedConfig.DefaultFields["service"])

	// 测试保存为YAML
	yamlPath := filepath.Join(tempDir, "saved_config.yaml")

	err = SaveToFile(config, yamlPath)
	require.NoError(t, err)

	// 重新加载并验证
	loadedYamlConfig, err := LoadFromFile(yamlPath)
	require.NoError(t, err)

	assert.Equal(t, config.Level, loadedYamlConfig.Level)
	assert.Equal(t, config.Format, loadedYamlConfig.Format)
	assert.Equal(t, config.FileConfig.Filename, loadedYamlConfig.FileConfig.Filename)
}

// 测试从环境变量加载配置
func TestFromEnv(t *testing.T) {
	// 重置全局变量，强制重新初始化
	v = nil
	globalConfig = nil
	envPrefix = ""
	listeners = nil
	configFile = ""
	initOnce = sync.Once{}

	// 设置环境变量（使用默认前缀VIRLOG_）
	os.Setenv("VIRLOG_LEVEL", "error")
	os.Setenv("VIRLOG_FORMAT", "console")
	os.Setenv("VIRLOG_OUTPUT", "file")
	os.Setenv("VIRLOG_DEVELOPMENT", "true")
	os.Setenv("VIRLOG_ENABLE_CALLER", "false")
	os.Setenv("VIRLOG_FILE_PATH", "/var/log/app.log")

	// 测试完成后清理环境变量
	defer func() {
		os.Unsetenv("VIRLOG_LEVEL")
		os.Unsetenv("VIRLOG_FORMAT")
		os.Unsetenv("VIRLOG_OUTPUT")
		os.Unsetenv("VIRLOG_DEVELOPMENT")
		os.Unsetenv("VIRLOG_ENABLE_CALLER")
		os.Unsetenv("VIRLOG_FILE_PATH")
	}()

	// 从环境变量加载配置
	config := GetConfig()

	// 验证配置
	assert.Equal(t, "error", config.Level)
	assert.Equal(t, "console", config.Format)
	assert.Equal(t, "file", config.Output)
	assert.True(t, config.Development)
	assert.False(t, config.EnableCaller)
	assert.Equal(t, "/var/log/app.log", config.FileConfig.Filename)
}
