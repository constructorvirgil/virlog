package vconfig

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/constructorvirgil/virlog/test/testutils"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// 示例配置结构体
type AppConfig struct {
	App struct {
		Name    string `json:"name" yaml:"name" toml:"name"`
		Version string `json:"version" yaml:"version" toml:"version"`
	} `json:"app" yaml:"app" toml:"app"`
	Server struct {
		Host string `json:"host" yaml:"host" toml:"host"`
		Port int    `json:"port" yaml:"port" toml:"port"`
	} `json:"server" yaml:"server" toml:"server"`
	Database struct {
		DSN      string `json:"dsn" yaml:"dsn" toml:"dsn"`
		MaxConns int    `json:"max_conns" yaml:"max_conns" toml:"max_conns"`
	} `json:"database" yaml:"database" toml:"database"`
	Log struct {
		Level  string `json:"level" yaml:"level" toml:"level"`
		Format string `json:"format" yaml:"format" toml:"format"`
	} `json:"log" yaml:"log" toml:"log"`
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
	testCases := []struct {
		name       string
		configType ConfigType
		extension  string
	}{
		{"YAML配置", YAML, ".yaml"},
		{"JSON配置", JSON, ".json"},
		{"TOML配置", TOML, ".toml"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试配置文件，使用随机文件名
			configFile := testutils.RandomTempFilename("test_config", tc.extension)

			// 使用规定的清理方式清理测试文件
			defer testutils.CleanTempFile(t, configFile)

			// 创建默认配置
			defaultConfig := newDefaultConfig()

			// 创建配置实例
			cfg, err := NewConfig(defaultConfig,
				WithConfigFile[AppConfig](configFile),
				WithConfigType[AppConfig](tc.configType))

			require.NoError(t, err)
			require.NotNil(t, cfg)

			// 验证默认配置已经写入文件并加载
			assert.Equal(t, defaultConfig.App.Name, cfg.GetData().App.Name)
			assert.Equal(t, defaultConfig.Server.Port, cfg.GetData().Server.Port)

			// 修改配置
			currentData := cfg.GetData()
			currentData.Server.Port = 9000
			err = cfg.Update(currentData)
			require.NoError(t, err)

			// 读取修改后的文件内容
			content, err := os.ReadFile(configFile)
			require.NoError(t, err)
			t.Logf("修改后的配置文件内容: \n%s", string(content))

			// 重新创建配置实例
			newCfg, err := NewConfig(AppConfig{},
				WithConfigFile[AppConfig](configFile),
				WithConfigType[AppConfig](tc.configType))
			require.NoError(t, err)
			defer newCfg.Close()

			// 根据不同的配置类型解析文件内容
			var parsedConfig AppConfig
			switch tc.configType {
			case JSON:
				err = json.Unmarshal(content, &parsedConfig)
			case YAML:
				err = yaml.Unmarshal(content, &parsedConfig)
			case TOML:
				err = toml.Unmarshal(content, &parsedConfig)
			}
			require.NoError(t, err)

			// 验证解析后的配置与实际配置一致
			assert.Equal(t, parsedConfig.Server.Port, newCfg.GetData().Server.Port)
			assert.Equal(t, parsedConfig.App.Name, newCfg.GetData().App.Name)
			assert.Equal(t, parsedConfig.Database.DSN, newCfg.GetData().Database.DSN)
			assert.Equal(t, parsedConfig.Log.Level, newCfg.GetData().Log.Level)
		})
	}
}

// 测试环境变量覆盖
func TestEnvVarOverride(t *testing.T) {
	testCases := []struct {
		name       string
		configType ConfigType
		extension  string
	}{
		{"YAML配置", YAML, ".yaml"},
		{"JSON配置", JSON, ".json"},
		{"TOML配置", TOML, ".toml"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试配置文件，使用随机文件名
			configFile := testutils.RandomTempFilename("test_env_config", tc.extension)

			// 使用规定的清理方式清理测试文件
			defer testutils.CleanTempFile(t, configFile)

			// 设置环境变量
			const portValue = "5000"
			os.Setenv("APP_SERVER_PORT", portValue)
			defer os.Unsetenv("APP_SERVER_PORT")

			// 创建配置实例，使用环境变量前缀
			cfg, err := NewConfig(newDefaultConfig(),
				WithConfigFile[AppConfig](configFile),
				WithConfigType[AppConfig](tc.configType),
				WithEnv[AppConfig](true),
				WithEnvPrefix[AppConfig]("APP"))

			require.NoError(t, err)

			// 验证环境变量覆盖了默认配置
			t.Logf("期望端口值: %s, 实际端口值: %d", portValue, cfg.GetData().Server.Port)
			assert.Equal(t, 5000, cfg.GetData().Server.Port)
		})
	}
}

// 测试配置变更回调
func TestConfigChangeCallback(t *testing.T) {
	testCases := []struct {
		name       string
		configType ConfigType
		extension  string
		newContent string
	}{
		{
			name:       "YAML配置",
			configType: YAML,
			extension:  ".yaml",
			newContent: `
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
`,
		},
		{
			name:       "JSON配置",
			configType: JSON,
			extension:  ".json",
			newContent: `{
  "app": {
    "name": "修改后的应用名称",
    "version": "1.0.1"
  },
  "server": {
    "host": "localhost",
    "port": 7000
  },
  "database": {
    "dsn": "postgres://user:password@localhost:5432/dbname",
    "max_conns": 10
  },
  "log": {
    "level": "debug",
    "format": "json"
  }
}`,
		},
		{
			name:       "TOML配置",
			configType: TOML,
			extension:  ".toml",
			newContent: `
[app]
name = "修改后的应用名称"
version = "1.0.1"

[server]
host = "localhost"
port = 7000

[database]
dsn = "postgres://user:password@localhost:5432/dbname"
max_conns = 10

[log]
level = "debug"
format = "json"
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试配置文件，使用随机文件名
			configFile := testutils.RandomTempFilename("test_callback_config", tc.extension)

			// 使用规定的清理方式清理测试文件
			defer testutils.CleanTempFile(t, configFile)

			// 创建配置实例
			cfg, err := NewConfig(newDefaultConfig(),
				WithConfigFile[AppConfig](configFile),
				WithConfigType[AppConfig](tc.configType))
			require.NoError(t, err)

			// 确认初始配置已写入
			initialContent, err := os.ReadFile(configFile)
			require.NoError(t, err)
			t.Logf("初始配置文件内容: \n%s", string(initialContent))

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

				// 验证所有预期的变更都存在
				expectedChanges := map[string]struct {
					oldValue interface{}
					newValue interface{}
				}{
					"app.name":    {"示例应用", "修改后的应用名称"},
					"app.version": {"1.0.0", "1.0.1"},
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

			// 写入新的配置内容
			err = os.WriteFile(configFile, []byte(tc.newContent), 0644)
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

// 测试配置变更检测
func TestConfigChangeDetection(t *testing.T) {
	// 创建测试配置文件
	configFile := testutils.RandomTempFilename("test_change_detection", ".yaml")
	defer testutils.CleanTempFile(t, configFile)

	// 创建配置实例
	cfg, err := NewConfig(newDefaultConfig(), WithConfigFile[AppConfig](configFile))
	require.NoError(t, err)

	// 创建变更记录通道
	changesCh := make(chan []ConfigChangedItem, 1)

	// 添加回调
	cfg.OnChange(func(e fsnotify.Event, changes []ConfigChangedItem) {
		t.Logf("检测到 %d 个配置变更", len(changes))
		for _, change := range changes {
			t.Logf("变更: %s, 旧值: %v, 新值: %v", change.Path, change.OldValue, change.NewValue)
		}
		changesCh <- changes
	})

	// 场景1: 修改基本类型
	newContent1 := `
app:
  name: "新应用名称"  # 修改基本类型
  version: "1.0.0"
server:
  host: "localhost"
  port: 8080
database:
  dsn: "postgres://user:password@localhost:5432/dbname"
  max_conns: 10
log:
  level: "info"
  format: "json"
`
	t.Log("写入第一个配置文件")
	err = os.WriteFile(configFile, []byte(newContent1), 0644)
	require.NoError(t, err)

	// 等待变更通知
	t.Log("等待第一个配置变更通知")
	var changes1 []ConfigChangedItem
	select {
	case changes1 = <-changesCh:
		t.Logf("收到第一个配置变更通知，包含 %d 个变更", len(changes1))
	case <-time.After(5 * time.Second):
		t.Fatal("等待配置变更通知超时1")
	}

	// 验证基本类型变更被正确检测
	require.NotEmpty(t, changes1, "应该检测到至少一个变更")

	// 确认app.name的变更
	found := false
	for _, change := range changes1 {
		if change.Path == "app.name" {
			found = true
			assert.Equal(t, "示例应用", change.OldValue)
			assert.Equal(t, "新应用名称", change.NewValue)
			break
		}
	}
	assert.True(t, found, "未检测到app.name的变更")

	// 等待一段时间确保文件系统监控稳定
	time.Sleep(1 * time.Second)

	// 测试方式2：直接使用FindConfigChanges函数测试变更检测
	t.Log("直接测试FindConfigChanges函数")

	// 创建两个不同的配置对象
	config1 := newDefaultConfig()
	config2 := newDefaultConfig()
	config2.App.Version = "2.0.0"
	config2.Server.Port = 9000
	config2.Log.Level = "debug"

	// 使用FindConfigChanges检测变更
	changes := findConfigChanges(config1, config2, "")
	t.Logf("FindConfigChanges检测到 %d 个变更", len(changes))
	for _, change := range changes {
		t.Logf("变更: %s, 旧值: %v, 新值: %v", change.Path, change.OldValue, change.NewValue)
	}

	// 验证变更检测
	expectedPaths := map[string]struct{}{
		"app.version": {},
		"server.port": {},
		"log.level":   {},
	}

	for _, change := range changes {
		if _, ok := expectedPaths[change.Path]; ok {
			delete(expectedPaths, change.Path)
			t.Logf("找到预期的变更: %s", change.Path)
		}
	}

	assert.Empty(t, expectedPaths, "有预期的变更未被检测到: %v", expectedPaths)
}

// TestEnvOnlyConfig 测试仅环境变量配置
func TestEnvOnlyConfig(t *testing.T) {
	// 定义测试用配置结构
	type TestConfig struct {
		App struct {
			Name string `json:"name" yaml:"name"`
			Port int    `json:"port" yaml:"port"`
		} `json:"app" yaml:"app"`
		Database struct {
			Host     string `json:"host" yaml:"host"`
			Port     int    `json:"port" yaml:"port"`
			Username string `json:"username" yaml:"username"`
			Password string `json:"password" yaml:"password"`
		} `json:"database" yaml:"database"`
	}

	// 设置默认配置
	defaultConfig := TestConfig{}
	defaultConfig.App.Name = "TestApp"
	defaultConfig.App.Port = 8080
	defaultConfig.Database.Host = "localhost"
	defaultConfig.Database.Port = 3306
	defaultConfig.Database.Username = "root"
	defaultConfig.Database.Password = "password"

	// 设置环境变量
	os.Setenv("TEST_APP_PORT", "9090")
	os.Setenv("TEST_DATABASE_HOST", "db.example.com")
	defer func() {
		os.Unsetenv("TEST_APP_PORT")
		os.Unsetenv("TEST_DATABASE_HOST")
	}()

	// 创建仅环境变量配置
	cfg, err := NewConfig(defaultConfig,
		WithEnvOnly[TestConfig](true),
		WithEnvPrefix[TestConfig]("TEST"))
	if err != nil {
		t.Fatalf("创建配置失败: %v", err)
	}
	defer cfg.Close()

	// 验证配置值
	data := cfg.GetData()
	if data.App.Name != "TestApp" {
		t.Errorf("App.Name 应为 %s, 实际为 %s", "TestApp", data.App.Name)
	}
	if data.App.Port != 9090 {
		t.Errorf("App.Port 应为 %d, 实际为 %d", 9090, data.App.Port)
	}
	if data.Database.Host != "db.example.com" {
		t.Errorf("Database.Host 应为 %s, 实际为 %s", "db.example.com", data.Database.Host)
	}

	// 测试配置变更回调
	callbackCalled := false
	cfg.OnChange(func(e fsnotify.Event, items []ConfigChangedItem) {
		callbackCalled = true
		if len(items) != 1 {
			t.Errorf("预期变更项数量为 1, 实际为 %d", len(items))
		}
		if items[0].Path != "database.port" {
			t.Errorf("预期变更项路径为 %s, 实际为 %s", "database.port", items[0].Path)
		}
		if items[0].OldValue.(int) != 3306 {
			t.Errorf("预期旧值为 %d, 实际为 %v", 3306, items[0].OldValue)
		}
		if items[0].NewValue.(int) != 5432 {
			t.Errorf("预期新值为 %d, 实际为 %v", 5432, items[0].NewValue)
		}
	})

	// 更新配置并检查回调是否被触发
	newData := data
	newData.Database.Port = 5432
	if err := cfg.Update(newData); err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	if !callbackCalled {
		t.Error("配置变更回调未被触发")
	}

	// 验证更新后的配置
	updatedData := cfg.GetData()
	if updatedData.Database.Port != 5432 {
		t.Errorf("更新后 Database.Port 应为 %d, 实际为 %d", 5432, updatedData.Database.Port)
	}
}
