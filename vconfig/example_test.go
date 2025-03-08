package vconfig

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
)

// 示例配置结构体
type AppConfig struct {
	App struct {
		Name    string `json:"name" yaml:"name"`
		Version string `json:"version" yaml:"version"`
	} `json:"app" yaml:"app"`
	Server struct {
		Host string `json:"host" yaml:"host"`
		Port int    `json:"port" yaml:"port"`
	} `json:"server" yaml:"server"`
	Database struct {
		DSN      string `json:"dsn" yaml:"dsn"`
		MaxConns int    `json:"max_conns" yaml:"max_conns"`
	} `json:"database" yaml:"database"`
	Log struct {
		Level  string `json:"level" yaml:"level"`
		Format string `json:"format" yaml:"format"`
	} `json:"log" yaml:"log"`
}

// 创建默认配置
func newDefaultConfig() AppConfig {
	config := AppConfig{}
	config.App.Name = "示例应用"
	config.App.Version = "1.0.0"
	config.Server.Host = "localhost"
	config.Server.Port = 8080
	config.Database.DSN = "postgres://user:password@localhost:5432/dbname"
	config.Database.MaxConns = 10
	config.Log.Level = "info"
	config.Log.Format = "json"
	return config
}

// 测试基本功能
func TestBasicConfig(t *testing.T) {
	configFile := "test_config.yaml"

	// 清理测试文件
	defer os.Remove(configFile)

	// 创建默认配置
	defaultConfig := newDefaultConfig()

	// 创建配置实例
	cfg, err := NewConfig(defaultConfig,
		WithConfigFile[AppConfig](configFile),
		WithConfigType[AppConfig](YAML),
		WithEnvPrefix[AppConfig]("APP"))

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// 验证默认配置已经写入文件并加载
	assert.Equal(t, defaultConfig.App.Name, cfg.Data.App.Name)
	assert.Equal(t, defaultConfig.Server.Port, cfg.Data.Server.Port)

	// 修改配置
	cfg.Data.Server.Port = 9000
	err = cfg.SaveConfig()
	assert.NoError(t, err)

	// 重新加载配置
	newCfg, err := NewConfig(AppConfig{}, WithConfigFile[AppConfig](configFile))
	assert.NoError(t, err)
	assert.Equal(t, 9000, newCfg.Data.Server.Port)
}

// 测试环境变量覆盖
func TestEnvVarOverride(t *testing.T) {
	configFile := "test_env_config.yaml"

	// 清理测试文件
	defer os.Remove(configFile)

	// 设置环境变量
	os.Setenv("APP_SERVER_PORT", "5000")
	defer os.Unsetenv("APP_SERVER_PORT")

	// 创建配置实例
	cfg, err := NewConfig(newDefaultConfig(),
		WithConfigFile[AppConfig](configFile),
		WithEnvPrefix[AppConfig]("APP"))

	assert.NoError(t, err)

	// 验证环境变量覆盖了默认配置
	assert.Equal(t, 5000, cfg.Data.Server.Port)
}

// 测试配置变更回调
func TestConfigChangeCallback(t *testing.T) {
	configFile := "test_callback_config.yaml"

	// 清理测试文件
	defer os.Remove(configFile)

	// 创建配置实例
	cfg, err := NewConfig(newDefaultConfig(), WithConfigFile[AppConfig](configFile))
	assert.NoError(t, err)

	// 标记是否回调被触发
	callbackTriggered := false

	// 添加回调函数
	cfg.OnChange(func(e fsnotify.Event) {
		callbackTriggered = true
		fmt.Println("配置发生变更:", e.Name)
	})

	// 修改配置文件
	newContent := `
app:
  name: "修改后的应用名称"
  version: "1.0.1"
server:
  host: "localhost"
  port: 7000
database:
  dsn: "postgres://user:password@localhost:5432/dbname"
  max_conns: 10
log:
  level: "debug"
  format: "json"
`

	// 写入新的配置内容
	err = os.WriteFile(configFile, []byte(newContent), 0644)
	assert.NoError(t, err)

	// 等待文件系统通知和回调被触发
	time.Sleep(1 * time.Second)

	// 验证配置已更新
	assert.Equal(t, "修改后的应用名称", cfg.Data.App.Name)
	assert.Equal(t, 7000, cfg.Data.Server.Port)
	assert.Equal(t, "debug", cfg.Data.Log.Level)

	// 验证回调被触发
	assert.True(t, callbackTriggered)
}

// 示例：如何使用配置模块
func ExampleNewConfig() {
	// 定义配置结构体
	type MyConfig struct {
		AppName string `yaml:"app_name"`
		Debug   bool   `yaml:"debug"`
		Server  struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		} `yaml:"server"`
	}

	// 创建默认配置
	defaultConfig := MyConfig{
		AppName: "我的应用",
		Debug:   true,
		Server: struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}{
			Host: "localhost",
			Port: 8080,
		},
	}

	// 创建配置实例
	cfg, err := NewConfig(defaultConfig,
		WithConfigFile[MyConfig]("config.yaml"),
		WithEnvPrefix[MyConfig]("MYAPP"))

	if err != nil {
		fmt.Println("初始化配置失败:", err)
		return
	}

	// 添加配置变更回调
	cfg.OnChange(func(e fsnotify.Event) {
		fmt.Println("配置已更新，需要重新加载某些组件")
	})

	// 获取配置
	config := cfg.Get()
	fmt.Println("应用名称:", config.AppName)
	fmt.Println("服务器端口:", config.Server.Port)

	// 输出:
	// 应用名称: 我的应用
	// 服务器端口: 8080
}
