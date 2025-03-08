package vconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// ConfigType 支持的配置文件类型
type ConfigType string

const (
	// JSON json格式配置文件
	JSON ConfigType = "json"
	// YAML yaml格式配置文件
	YAML ConfigType = "yaml"
	// TOML toml格式配置文件
	TOML ConfigType = "toml"
)

// 配置项变更回调函数类型
type OnConfigChangeCallback func(e fsnotify.Event)

// Config 通用配置结构体
type Config[T any] struct {
	// 配置数据
	data T
	// viper实例
	v *viper.Viper
	// 配置文件路径
	configFile string
	// 配置文件类型
	configType ConfigType
	// 是否启用环境变量
	enableEnv bool
	// 环境变量前缀
	envPrefix string
	// 配置文件变更回调函数列表
	changeCallbacks []OnConfigChangeCallback
	// 保护回调函数列表的互斥锁
	callbackMu sync.RWMutex
	// 上次修改时间，用于防止短时间内重复触发回调
	lastModTime time.Time
	// 防抖时间
	debounceTime time.Duration
}

// ConfigOption 配置选项函数
type ConfigOption[T any] func(*Config[T])

// WithConfigFile 设置配置文件路径
func WithConfigFile[T any](configFile string) ConfigOption[T] {
	return func(c *Config[T]) {
		c.configFile = configFile
	}
}

// WithConfigType 设置配置文件类型
func WithConfigType[T any](configType ConfigType) ConfigOption[T] {
	return func(c *Config[T]) {
		c.configType = configType
	}
}

// WithEnvPrefix 启用环境变量并设置前缀
func WithEnvPrefix[T any](prefix string) ConfigOption[T] {
	return func(c *Config[T]) {
		c.enableEnv = true
		c.envPrefix = prefix
	}
}

// WithDebounceTime 设置防抖时间
func WithDebounceTime[T any](duration time.Duration) ConfigOption[T] {
	return func(c *Config[T]) {
		c.debounceTime = duration
	}
}

// OnChange 添加配置文件变更回调函数
func (c *Config[T]) OnChange(callback OnConfigChangeCallback) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.changeCallbacks = append(c.changeCallbacks, callback)
}

// 触发所有回调函数
func (c *Config[T]) triggerCallbacks(e fsnotify.Event) {
	now := time.Now()
	// 防抖：如果与上次修改时间间隔小于设定的防抖时间，则忽略
	if now.Sub(c.lastModTime) < c.debounceTime {
		return
	}
	c.lastModTime = now

	c.callbackMu.RLock()
	defer c.callbackMu.RUnlock()
	for _, callback := range c.changeCallbacks {
		if callback != nil {
			callback(e)
		}
	}
}

// 重新加载配置
func (c *Config[T]) reload() error {
	// 确保文件存在
	if _, err := os.Stat(c.configFile); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %w", err)
	}

	// 重新读取配置文件内容
	fileBytes, err := os.ReadFile(c.configFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 创建新的viper实例读取配置
	v := viper.New()
	v.SetConfigType(string(c.configType))

	// 从字节流读取配置
	if err := v.ReadConfig(bytes.NewBuffer(fileBytes)); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 应用环境变量配置
	if c.enableEnv {
		v.SetEnvPrefix(c.envPrefix)
		v.AutomaticEnv()
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

		// 绑定所有键到环境变量
		for _, key := range v.AllKeys() {
			bindKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
			if err := v.BindEnv(key, c.envPrefix+"_"+bindKey); err != nil {
				return fmt.Errorf("绑定环境变量失败: %w", err)
			}
		}
	}

	// 将读取的配置应用到当前的viper实例
	allSettings := v.AllSettings()
	for k, val := range allSettings {
		c.v.Set(k, val)
	}

	// 将配置解析到结构体
	if err := c.v.Unmarshal(&c.data); err != nil {
		return fmt.Errorf("解析配置到结构体失败: %w", err)
	}

	return nil
}

// 监听配置文件变更
func (c *Config[T]) watchConfig() {
	c.v.OnConfigChange(func(e fsnotify.Event) {
		if e.Op == fsnotify.Write {
			// 重新加载配置
			err := c.reload()
			if err != nil {
				fmt.Printf("配置文件变更后重新加载失败: %v\n", err)
				return
			}
			// 触发回调
			c.triggerCallbacks(e)
		}
	})
	c.v.WatchConfig()
}

// NewConfig 创建一个新的配置实例
func NewConfig[T any](defaultConfig T, options ...ConfigOption[T]) (*Config[T], error) {
	config := &Config[T]{
		data:         defaultConfig,
		v:            viper.New(),
		configType:   YAML,                   // 默认YAML格式
		debounceTime: 500 * time.Millisecond, // 默认防抖时间500ms
		lastModTime:  time.Time{},
	}

	// 应用选项
	for _, option := range options {
		option(config)
	}

	// 设置配置文件类型
	config.v.SetConfigType(string(config.configType))

	// 如果配置了配置文件路径
	if config.configFile != "" {
		// 设置配置文件
		configDir := filepath.Dir(config.configFile)
		configName := filepath.Base(config.configFile)
		// 去掉扩展名
		ext := filepath.Ext(configName)
		if ext != "" {
			configName = configName[:len(configName)-len(ext)]
		}

		// 如果配置文件目录不存在，创建目录
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return nil, fmt.Errorf("创建配置目录失败: %w", err)
			}
		}

		config.v.AddConfigPath(configDir)
		config.v.SetConfigName(configName)

		// 检查配置文件是否存在
		configExists := true
		if _, err := os.Stat(config.configFile); os.IsNotExist(err) {
			configExists = false
		}

		// 首先将默认配置加载到viper中
		// 使用viper提供的支持结构体到map的转换方法
		if err := config.bindStruct(defaultConfig); err != nil {
			return nil, fmt.Errorf("绑定默认配置失败: %w", err)
		}

		// 设置环境变量覆盖
		if config.enableEnv {
			config.v.SetEnvPrefix(config.envPrefix)
			config.v.AutomaticEnv()
			// 支持嵌套结构体的环境变量，如APP_SERVER_PORT
			config.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		}

		// 如果配置文件不存在，则创建
		if !configExists {
			// 保存配置到文件
			if err := config.v.WriteConfigAs(config.configFile); err != nil {
				return nil, fmt.Errorf("创建默认配置文件失败: %w", err)
			}
		} else {
			// 配置文件存在，加载已有配置
			fileBytes, err := os.ReadFile(config.configFile)
			if err != nil {
				return nil, fmt.Errorf("读取配置文件失败: %w", err)
			}

			// 创建临时viper实例读取配置文件
			tempViper := viper.New()
			tempViper.SetConfigType(string(config.configType))

			// 从字节流读取配置
			if err := tempViper.ReadConfig(bytes.NewBuffer(fileBytes)); err != nil {
				return nil, fmt.Errorf("解析配置文件失败: %w", err)
			}

			// 将读取的配置应用到当前的viper实例
			allSettings := tempViper.AllSettings()
			for k, val := range allSettings {
				config.v.Set(k, val)
			}
		}

		// 如果启用了环境变量，应用环境变量覆盖
		if config.enableEnv {
			// 绑定所有键到环境变量
			for _, key := range config.v.AllKeys() {
				envKey := config.envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
				// 检查环境变量是否设置
				if val := os.Getenv(envKey); val != "" {
					// 根据值的类型设置
					if val == "true" || val == "false" {
						config.v.Set(key, val == "true")
					} else if intVal, err := strconv.Atoi(val); err == nil {
						config.v.Set(key, intVal)
					} else if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
						config.v.Set(key, floatVal)
					} else {
						config.v.Set(key, val)
					}
				}
			}
		}

		// 将配置解析到结构体
		if err := config.v.Unmarshal(&config.data); err != nil {
			return nil, fmt.Errorf("解析配置到结构体失败: %w", err)
		}

		// 监听配置文件变更
		config.watchConfig()
	} else {
		return nil, errors.New("未指定配置文件路径")
	}

	return config, nil
}

// bindStruct 将结构体绑定到配置
func (c *Config[T]) bindStruct(data T) error {
	// 使用反射将结构体转换为map
	// 我们可以利用 viper 的能力先将数据序列化为 JSON，然后再读取回来
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化结构体失败: %w", err)
	}

	// 创建临时的 viper 实例
	tempViper := viper.New()
	tempViper.SetConfigType("json")

	// 从 JSON 读取数据
	if err := tempViper.ReadConfig(bytes.NewBuffer(jsonBytes)); err != nil {
		return fmt.Errorf("将JSON读取到viper失败: %w", err)
	}

	// 获取所有设置并应用到主 viper 实例
	settings := tempViper.AllSettings()
	for k, v := range settings {
		c.v.Set(k, v)
	}

	return nil
}

// SaveConfig 保存配置到文件
func (c *Config[T]) SaveConfig() error {
	// 先将当前结构体绑定到viper
	if err := c.bindStruct(c.data); err != nil {
		return fmt.Errorf("绑定结构体到配置失败: %w", err)
	}

	// 保存到文件
	if err := c.v.WriteConfigAs(c.configFile); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// GetViper 获取底层的viper实例
func (c *Config[T]) GetViper() *viper.Viper {
	return c.v
}

// GetData 获取配置数据
func (c *Config[T]) GetData() T {
	return c.data
}

// Update 更新配置数据并保存
func (c *Config[T]) Update(data T) error {
	c.data = data
	return c.SaveConfig()
}
