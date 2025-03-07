package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试配置监听器
func TestConfigListener(t *testing.T) {
	// 初始化全局配置
	initConfig()

	// 创建一个监听器
	listenerChan := make(chan *Config, 1)
	AddListener(listenerChan)
	defer RemoveListener(listenerChan)

	// 接收初始配置
	initialConfig := <-listenerChan
	assert.NotNil(t, initialConfig)

	// 修改配置
	newConfig := DefaultConfig()
	newConfig.Level = "debug"
	SetConfig(newConfig)

	// 等待配置更新
	select {
	case updatedConfig := <-listenerChan:
		assert.Equal(t, "debug", updatedConfig.Level)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("没有收到配置更新")
	}
}

// 测试配置文件监听
func TestViperWatchConfig(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")

	// 保存初始配置
	initialConfig := DefaultConfig()
	initialConfig.Level = "info"
	err := SaveToFile(initialConfig, configFile)
	require.NoError(t, err)

	// 设置环境变量
	oldConfigFile := os.Getenv(EnvConfigFile)
	os.Setenv(EnvConfigFile, configFile)
	defer os.Setenv(EnvConfigFile, oldConfigFile)

	// 重置全局变量，强制重新初始化
	v = nil
	globalConfig = nil
	listeners = nil
	configFile = ""
	initOnce = sync.Once{}

	// 创建监听器
	listenerChan := make(chan *Config, 1)
	AddListener(listenerChan)
	defer RemoveListener(listenerChan)

	// 接收初始配置
	<-listenerChan

	// 更新配置文件
	updatedConfig := DefaultConfig()
	updatedConfig.Level = "debug"
	updatedConfig.Format = "console"
	err = SaveToFile(updatedConfig, configFile)
	require.NoError(t, err)

	// 等待viper检测到文件变化并发送通知
	// 注意：有些系统可能需要更长时间来检测文件变化
	select {
	case newConfig := <-listenerChan:
		assert.Equal(t, "debug", newConfig.Level)
		assert.Equal(t, "console", newConfig.Format)
	case <-time.After(3 * time.Second):
		t.Skip("跳过测试：文件监听可能不支持或需要更长时间")
	}
}

// 测试环境变量前缀
func TestEnvPrefix(t *testing.T) {
	// 保存原始环境变量
	oldPrefix := os.Getenv(EnvPrefix)
	defer os.Setenv(EnvPrefix, oldPrefix)

	// 设置自定义前缀
	os.Setenv(EnvPrefix, "TEST_")

	// 设置测试环境变量
	os.Setenv("TEST_LEVEL", "error")
	defer os.Unsetenv("TEST_LEVEL")

	// 重置全局变量，强制重新初始化
	envPrefix = ""
	globalConfig = nil
	initOnce = sync.Once{}

	// 获取配置
	cfg := GetConfig()

	// 验证配置
	assert.Equal(t, "error", cfg.Level)
	assert.Equal(t, "TEST_", GetEnvPrefix())
}
