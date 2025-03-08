package vconfig

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 生成随机临时文件名
func randomTempFilename(prefix, suffix string) string {
	rand.Seed(time.Now().UnixNano())
	randNum := rand.Intn(100000)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d_%d%s", prefix, timestamp, randNum, suffix)
}

// 延时清理临时文件
func cleanTempFile(t *testing.T, tempFile string) {
	// 先尝试直接删除
	err := os.Remove(tempFile)
	if err != nil {
		// 进程属性设置
		procAttr := &os.ProcAttr{
			Files: []*os.File{nil, nil, nil}, // 标准输入、输出、错误均设置为nil
			Dir:   "",                        // 使用当前目录
		}

		var executable string
		var args []string

		switch runtime.GOOS {
		case "windows":
			// Windows系统
			executable, err = exec.LookPath("powershell.exe")
			if err != nil {
				t.Logf("Failed to find executable %s: %v", executable, err)
				return
			}
			t.Logf("Executable: %s", executable)
			// 使用Start-Sleep命令等待2秒后再删除
			args = []string{"-Command", fmt.Sprintf("Start-Sleep -Seconds 2; Remove-Item -Path '%s' -Force", tempFile)}
		case "darwin", "linux", "freebsd", "openbsd", "netbsd":
			// Unix系统
			executable = "/bin/sh"
			// 使用sleep命令等待2秒后再删除
			args = []string{"-c", fmt.Sprintf("sleep 2 && rm -f \"%s\"", tempFile)}
		default:
			t.Logf("Unsupported OS: %s", runtime.GOOS)
			return
		}

		// 启动进程
		_, err := os.StartProcess(executable, append([]string{executable}, args...), procAttr)
		if err != nil {
			t.Logf("Start process failed: %v", err)
			return
		}

		t.Logf("File locked, scheduled for deletion by separate process")
	}
}

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
	// 创建测试配置文件，使用随机文件名
	configFile := randomTempFilename("test_config", ".yaml")

	// 使用规定的清理方式清理测试文件
	defer cleanTempFile(t, configFile)

	// 创建默认配置
	defaultConfig := newDefaultConfig()

	// 创建配置实例
	cfg, err := NewConfig(defaultConfig,
		WithConfigFile[AppConfig](configFile),
		WithConfigType[AppConfig](YAML))

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// 验证默认配置已经写入文件并加载
	assert.Equal(t, defaultConfig.App.Name, cfg.data.App.Name)
	assert.Equal(t, defaultConfig.Server.Port, cfg.data.Server.Port)

	// 修改配置
	cfg.data.Server.Port = 9000
	err = cfg.SaveConfig()
	require.NoError(t, err)

	// 读取修改后的文件内容
	content, err := os.ReadFile(configFile)
	require.NoError(t, err)
	t.Logf("修改后的配置文件内容: \n%s", string(content))

	// 重新创建配置实例
	newCfg, err := NewConfig(AppConfig{}, WithConfigFile[AppConfig](configFile))
	require.NoError(t, err)
	assert.Equal(t, 9000, newCfg.data.Server.Port)
}

// 测试环境变量覆盖
func TestEnvVarOverride(t *testing.T) {
	// 创建测试配置文件，使用随机文件名
	configFile := randomTempFilename("test_env_config", ".yaml")

	// 使用规定的清理方式清理测试文件
	defer cleanTempFile(t, configFile)

	// 设置环境变量
	const portValue = "5000"
	os.Setenv("APP_SERVER_PORT", portValue)
	defer os.Unsetenv("APP_SERVER_PORT")

	// 创建配置实例，使用环境变量前缀
	cfg, err := NewConfig(newDefaultConfig(),
		WithConfigFile[AppConfig](configFile),
		WithEnvPrefix[AppConfig]("APP"))

	require.NoError(t, err)

	// 验证环境变量覆盖了默认配置
	t.Logf("期望端口值: %s, 实际端口值: %d", portValue, cfg.data.Server.Port)
	assert.Equal(t, 5000, cfg.data.Server.Port)
}

// 测试配置变更回调
func TestConfigChangeCallback(t *testing.T) {
	// 创建测试配置文件，使用随机文件名
	configFile := randomTempFilename("test_callback_config", ".yaml")

	// 使用规定的清理方式清理测试文件
	defer cleanTempFile(t, configFile)

	// 创建配置实例
	cfg, err := NewConfig(newDefaultConfig(), WithConfigFile[AppConfig](configFile))
	require.NoError(t, err)

	// 确认初始配置已写入
	initialContent, err := os.ReadFile(configFile)
	require.NoError(t, err)
	t.Logf("初始配置文件内容: \n%s", string(initialContent))

	// 标记是否回调被触发
	callbackTriggered := false
	callbackCh := make(chan bool)

	// 添加回调函数
	cfg.OnChange(func(e fsnotify.Event) {
		callbackTriggered = true
		t.Logf("配置发生变更: %s", e.Name)
		callbackCh <- true
	})

	// 准备新的配置内容
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
	require.NoError(t, err)

	// 等待回调被触发或超时
	select {
	case <-callbackCh:
		// 回调被触发
	case <-time.After(3 * time.Second):
		t.Fatal("等待配置变更回调超时")
	}

	// 验证回调被触发
	assert.True(t, callbackTriggered)

	// 验证配置已更新
	assert.Equal(t, "修改后的应用名称", cfg.data.App.Name)
	assert.Equal(t, 7000, cfg.data.Server.Port)
	assert.Equal(t, "debug", cfg.data.Log.Level)
}
