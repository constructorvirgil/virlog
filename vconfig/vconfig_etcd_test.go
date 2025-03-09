package vconfig

import (
	"context"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/virlog/test/testutils"
	"gopkg.in/yaml.v3"
)

// 测试ETCD基本功能
func TestETCDConfig(t *testing.T) {
	// 创建ETCD配置
	etcdConfig := DefaultETCDConfig()
	etcdConfig.Key = "/test/config"

	// 清理ETCD中的配置
	client, err := newETCDClient(etcdConfig)
	require.NoError(t, err)
	_, err = client.client.Delete(context.Background(), etcdConfig.Key)
	require.NoError(t, err)
	client.close()

	// 创建默认配置
	defaultConfig := newDefaultConfig()

	// 创建配置实例
	cfg, err := NewConfig(defaultConfig,
		WithETCDConfig[AppConfig](etcdConfig))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	defer cfg.Close()

	// 验证默认配置已经写入ETCD并加载
	assert.Equal(t, defaultConfig.App.Name, cfg.GetData().App.Name)
	assert.Equal(t, defaultConfig.Server.Port, cfg.GetData().Server.Port)

	// 修改配置
	currentData := cfg.GetData()
	currentData.Server.Port = 9000
	err = cfg.Update(currentData)
	require.NoError(t, err)

	// 重新创建配置实例
	newCfg, err := NewConfig(AppConfig{}, WithETCDConfig[AppConfig](etcdConfig))
	require.NoError(t, err)
	defer newCfg.Close()

	assert.Equal(t, 9000, newCfg.GetData().Server.Port)
}

// 测试ETCD配置变更回调
func TestETCDConfigChangeCallback(t *testing.T) {
	// 创建ETCD配置
	etcdConfig := DefaultETCDConfig()
	etcdConfig.Key = "/test/callback/config"

	// 清理ETCD中的配置
	client, err := newETCDClient(etcdConfig)
	require.NoError(t, err)
	_, err = client.client.Delete(context.Background(), etcdConfig.Key)
	require.NoError(t, err)
	client.close()

	// 创建默认配置
	defaultConfig := newDefaultConfig()

	// 创建配置实例
	cfg, err := NewConfig(defaultConfig, WithETCDConfig[AppConfig](etcdConfig))
	require.NoError(t, err)
	defer cfg.Close()

	// 标记是否回调被触发
	callbackTriggered := false
	callbackCh := make(chan bool)

	// 添加回调函数
	cfg.OnChange(func(e fsnotify.Event, changedItems []ConfigChangedItem) {
		callbackTriggered = true
		t.Logf("配置发生变更: %s", e.Name)

		// 打印变动的配置项
		t.Logf("变更项数量: %d", len(changedItems))
		for _, item := range changedItems {
			t.Logf("变更项: %s, 旧值: %v, 新值: %v", item.Path, item.OldValue, item.NewValue)
		}

		callbackCh <- true
	})

	// 修改配置
	currentData := cfg.GetData()
	currentData.App.Name = "修改后的应用名称"
	currentData.Server.Port = 7000
	currentData.Log.Level = "debug"

	err = cfg.Update(currentData)
	require.NoError(t, err)

	// 等待回调被触发
	<-callbackCh

	// 验证回调被触发
	assert.True(t, callbackTriggered)

	// 验证配置已更新
	assert.Equal(t, "修改后的应用名称", cfg.GetData().App.Name)
	assert.Equal(t, 7000, cfg.GetData().Server.Port)
	assert.Equal(t, "debug", cfg.GetData().Log.Level)

	// 从ETCD直接查询键值进行比较
	client, err = newETCDClient(etcdConfig)
	require.NoError(t, err)
	defer client.close()

	// 获取ETCD中的配置数据
	data, err := client.get()
	require.NoError(t, err)

	// 解析ETCD中的配置
	var remoteETCDConfig AppConfig
	err = yaml.Unmarshal(data, &remoteETCDConfig)
	require.NoError(t, err)

	// 验证ETCD中的配置与内存中的配置一致
	assert.Equal(t, "修改后的应用名称", remoteETCDConfig.App.Name)
	assert.Equal(t, 7000, remoteETCDConfig.Server.Port)
	assert.Equal(t, "debug", remoteETCDConfig.Log.Level)
}

// 测试ETCD认证
func TestETCDAuth(t *testing.T) {
	// 创建ETCD配置
	etcdConfig := DefaultETCDConfig()
	etcdConfig.Key = "/test/auth/config"
	etcdConfig.Username = "test"
	etcdConfig.Password = "test123"

	// 清理ETCD中的配置
	client, err := newETCDClient(etcdConfig)
	if err == nil {
		_, err = client.client.Delete(context.Background(), etcdConfig.Key)
		require.NoError(t, err)
		client.close()
	}

	// 创建配置实例
	cfg, err := NewConfig(newDefaultConfig(),
		WithETCDConfig[AppConfig](etcdConfig))

	// 如果ETCD没有启用认证，这里会失败
	if err != nil {
		t.Skipf("ETCD认证测试跳过: %v", err)
		return
	}
	defer cfg.Close()

	// 验证配置已加载
	assert.NotEmpty(t, cfg.GetData().App.Name)
}

// 测试同时使用配置文件和ETCD
func TestConfigSourceConflict(t *testing.T) {
	// 创建测试配置文件
	configFile := testutils.RandomTempFilename("test_conflict", ".yaml")
	defer testutils.CleanTempFile(t, configFile)

	// 创建ETCD配置
	etcdConfig := DefaultETCDConfig()

	// 尝试同时使用配置文件和ETCD
	_, err := NewConfig(newDefaultConfig(),
		WithConfigFile[AppConfig](configFile),
		WithETCDConfig[AppConfig](etcdConfig))

	// 应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能同时使用配置文件和ETCD")
}

// 测试ETCD TLS连接
func TestETCDTLS(t *testing.T) {
	// 创建ETCD配置
	etcdConfig := DefaultETCDConfig()
	etcdConfig.Key = "/test/tls/config"
	etcdConfig.TLS = &TLSConfig{
		CertFile:      "test-cert.pem",
		KeyFile:       "test-key.pem",
		TrustedCAFile: "test-ca.pem",
	}

	// 清理ETCD中的配置
	client, err := newETCDClient(etcdConfig)
	if err == nil {
		_, err = client.client.Delete(context.Background(), etcdConfig.Key)
		require.NoError(t, err)
		client.close()
	}

	// 创建配置实例
	cfg, err := NewConfig(newDefaultConfig(),
		WithETCDConfig[AppConfig](etcdConfig))

	// 如果没有TLS证书，这里会失败
	if err != nil {
		t.Skipf("ETCD TLS测试跳过: %v", err)
		return
	}
	defer cfg.Close()

	// 验证配置已加载
	assert.NotEmpty(t, cfg.GetData().App.Name)
}

// TestETCDConfigWithDifferentFormats 测试不同格式的ETCD配置
func TestETCDConfigWithDifferentFormats(t *testing.T) {
	testCases := []struct {
		name       string
		configType ConfigType
	}{
		{"JSON格式", JSON},
		{"YAML格式", YAML},
		{"TOML格式", TOML},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建ETCD配置
			etcdConfig := DefaultETCDConfig()
			etcdConfig.Key = "/test/config/different-formats"

			// 清理ETCD中的配置
			client, err := newETCDClient(etcdConfig)
			require.NoError(t, err)
			_, err = client.client.Delete(context.Background(), etcdConfig.Key)
			require.NoError(t, err)
			client.close()

			// 创建默认配置
			defaultConfig := newDefaultConfig()

			// 创建配置实例
			cfg, err := NewConfig(defaultConfig,
				WithETCDConfig[AppConfig](etcdConfig),
				WithConfigType[AppConfig](tc.configType))
			require.NoError(t, err)
			require.NotNil(t, cfg)
			defer cfg.Close()

			// 验证默认配置已经写入ETCD并加载
			assert.Equal(t, defaultConfig.App.Name, cfg.GetData().App.Name)
			assert.Equal(t, defaultConfig.Server.Port, cfg.GetData().Server.Port)

			// 修改配置
			currentData := cfg.GetData()
			currentData.Server.Port = 9000
			currentData.App.Name = "修改后的应用名称"
			err = cfg.Update(currentData)
			require.NoError(t, err)

			// 重新创建配置实例
			newCfg, err := NewConfig(AppConfig{},
				WithETCDConfig[AppConfig](etcdConfig),
				WithConfigType[AppConfig](tc.configType))
			require.NoError(t, err)
			defer newCfg.Close()

			// 验证配置已更新
			assert.Equal(t, "修改后的应用名称", newCfg.GetData().App.Name)
			assert.Equal(t, 9000, newCfg.GetData().Server.Port)
		})
	}
}

// TestETCDConfigChangeCallbackWithDifferentFormats 测试不同格式的ETCD配置变更回调
func TestETCDConfigChangeCallbackWithDifferentFormats(t *testing.T) {
	testCases := []struct {
		name       string
		configType ConfigType
	}{
		{"JSON格式", JSON},
		// {"YAML格式", YAML},
		// {"TOML格式", TOML},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建ETCD配置
			etcdConfig := DefaultETCDConfig()
			etcdConfig.Key = "/test/config/different-formats-changes"

			// 清理ETCD中的配置
			client, err := newETCDClient(etcdConfig)
			require.NoError(t, err)
			_, err = client.client.Delete(context.Background(), etcdConfig.Key)
			require.NoError(t, err)
			client.close()

			// 创建配置实例
			cfg, err := NewConfig(newDefaultConfig(),
				WithETCDConfig[AppConfig](etcdConfig),
				WithConfigType[AppConfig](tc.configType))
			require.NoError(t, err)
			require.NotNil(t, cfg)
			defer cfg.Close()

			// 标记是否回调被触发
			callbackTriggered := false
			callbackCh := make(chan bool)

			// 添加回调函数
			cfg.OnChange(func(e fsnotify.Event, changedItems []ConfigChangedItem) {
				callbackTriggered = true
				t.Logf("配置发生变更")

				// 打印变动的配置项
				t.Logf("变更项数量: %d", len(changedItems))
				for _, item := range changedItems {
					t.Logf("变更项: %s, 旧值: %v, 新值: %v", item.Path, item.OldValue, item.NewValue)
				}

				// 验证所有预期的变更都存在
				expectedChanges := map[string]struct {
					oldValue interface{}
					newValue interface{}
				}{
					"app.name":    {"示例应用", "修改后的应用名称"},
					"app.version": {"1.0.0", "2.0.0"},
					"server.port": {8080, 7000},
					"log.level":   {"info", "debug"},
				}

				assert.Equal(t, len(expectedChanges), len(changedItems), "变更项数量不匹配")

				for _, item := range changedItems {
					expected, ok := expectedChanges[item.Path]
					assert.True(t, ok, "未预期的变更项: %s", item.Path)
					assert.Equal(t, expected.oldValue, item.OldValue, "变更项 %s 的旧值不匹配", item.Path)
					assert.Equal(t, expected.newValue, item.NewValue, "变更项 %s 的新值不匹配", item.Path)
				}

				callbackCh <- true
			})

			// 修改配置
			newConfig := cfg.GetData()
			newConfig.App.Name = "修改后的应用名称"
			newConfig.App.Version = "2.0.0"
			newConfig.Server.Port = 7000
			newConfig.Log.Level = "debug"

			// 更新配置
			err = cfg.Update(newConfig)
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
			assert.Equal(t, "修改后的应用名称", cfg.GetData().App.Name)
			assert.Equal(t, 7000, cfg.GetData().Server.Port)
			assert.Equal(t, "debug", cfg.GetData().Log.Level)
		})
	}
}
