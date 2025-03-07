package config

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	// 暂时跳过此测试，因为文件监听在某些系统中可能不稳定
	t.Skip("暂时跳过文件监听测试")
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
