package vconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// ConfigChangedItem 配置变更项
type ConfigChangedItem struct {
	// 配置路径，使用点号分隔，如 "app.server.port"
	Path string
	// 旧值
	OldValue interface{}
	// 新值
	NewValue interface{}
}

// 配置项变更回调函数类型
type OnConfigChangeCallback func(e fsnotify.Event, changedItems []ConfigChangedItem)

// Config 通用配置结构体
type Config[T any] struct {
	// 配置数据
	data T
	// 旧配置数据，用于比较变化
	oldData T
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
	// 是否已关闭
	closed bool
	// 保护closed字段的互斥锁
	closedMu sync.RWMutex
	// ETCD配置
	etcdConfig *ETCDConfig
	// ETCD客户端
	etcdClient *etcdClient
}

// OnChange 添加配置文件变更回调函数
func (c *Config[T]) OnChange(callback OnConfigChangeCallback) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.changeCallbacks = append(c.changeCallbacks, callback)
}

// 触发所有回调函数
func (c *Config[T]) triggerCallbacks(e fsnotify.Event) {
	// 检查配置是否已关闭
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return
	}
	c.closedMu.RUnlock()

	now := time.Now()
	// 防抖：如果与上次修改时间间隔小于设定的防抖时间，则忽略
	if now.Sub(c.lastModTime) < c.debounceTime {
		return
	}
	c.lastModTime = now

	// 查找配置变更项
	changedItems := findConfigChanges(c.oldData, c.data, "")

	c.callbackMu.RLock()
	defer c.callbackMu.RUnlock()
	for _, callback := range c.changeCallbacks {
		if callback != nil {
			callback(e, changedItems)
		}
	}
}

// 克隆配置数据
func cloneConfig[T any](src T) T {
	var dst T
	data, err := json.Marshal(src)
	if err != nil {
		return dst
	}
	json.Unmarshal(data, &dst)
	return dst
}

// 重新加载配置
func (c *Config[T]) reload() error {
	// 检查配置是否已关闭
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return errors.New("配置已关闭")
	}
	c.closedMu.RUnlock()

	// 确保文件存在
	if _, err := os.Stat(c.configFile); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %w", err)
	}

	// 在重载前保存当前配置用于比较
	c.oldData = cloneConfig(c.data)

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
		// 检查配置是否已关闭
		c.closedMu.RLock()
		if c.closed {
			c.closedMu.RUnlock()
			return
		}
		c.closedMu.RUnlock()

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
		oldData:      cloneConfig(defaultConfig),
		v:            viper.New(),
		configType:   YAML,                   // 默认YAML格式
		debounceTime: 500 * time.Millisecond, // 默认防抖时间500ms
		lastModTime:  time.Time{},
	}

	// 应用选项
	for _, option := range options {
		option(config)
	}

	// 检查配置源
	if config.configFile != "" && config.etcdConfig != nil {
		return nil, fmt.Errorf("不能同时使用配置文件和ETCD")
	}

	if config.configFile == "" && config.etcdConfig == nil {
		return nil, fmt.Errorf("必须指定配置文件或ETCD配置")
	}

	// 根据配置源初始化
	if config.configFile != "" {
		// 使用配置文件
		if err := config.initWithFile(); err != nil {
			return nil, err
		}
	} else {
		// 使用ETCD
		if err := config.initWithETCD(); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// initWithFile 使用配置文件初始化
func (c *Config[T]) initWithFile() error {
	// 设置配置文件类型
	c.v.SetConfigType(string(c.configType))

	// 设置配置文件
	configDir := filepath.Dir(c.configFile)
	configName := filepath.Base(c.configFile)
	// 去掉扩展名
	ext := filepath.Ext(configName)
	if ext != "" {
		configName = configName[:len(configName)-len(ext)]
	}

	// 如果配置文件目录不存在，创建目录
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %w", err)
		}
	}

	c.v.AddConfigPath(configDir)
	c.v.SetConfigName(configName)

	// 检查配置文件是否存在
	configExists := true
	if _, err := os.Stat(c.configFile); os.IsNotExist(err) {
		configExists = false
	}

	// 首先将默认配置加载到viper中
	if err := c.bindStruct(c.data); err != nil {
		return fmt.Errorf("绑定默认配置失败: %w", err)
	}

	// 设置环境变量覆盖
	if c.enableEnv {
		c.v.SetEnvPrefix(c.envPrefix)
		c.v.AutomaticEnv()
		c.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	}

	// 如果配置文件不存在，则创建
	if !configExists {
		if err := c.v.WriteConfigAs(c.configFile); err != nil {
			return fmt.Errorf("创建默认配置文件失败: %w", err)
		}
	} else {
		// 配置文件存在，加载已有配置
		if err := c.loadFromFile(); err != nil {
			return err
		}
	}

	// 监听配置文件变更
	c.watchConfig()

	return nil
}

// initWithETCD 使用ETCD初始化
func (c *Config[T]) initWithETCD() error {
	// 创建ETCD客户端
	client, err := newETCDClient(c.etcdConfig)
	if err != nil {
		return fmt.Errorf("创建ETCD客户端失败: %w", err)
	}
	c.etcdClient = client

	// 从ETCD加载配置
	if err := loadConfigFromETCD(c.etcdClient, &c.data); err != nil {
		return fmt.Errorf("从ETCD加载配置失败: %w", err)
	}

	// 如果ETCD中没有配置，保存默认配置
	emptyJSON, _ := json.Marshal(*new(T))
	currentJSON, _ := json.Marshal(c.data)
	if string(currentJSON) == string(emptyJSON) {
		c.data = c.oldData
		if err := saveConfigToETCD(c.etcdClient, c.data); err != nil {
			return fmt.Errorf("保存默认配置到ETCD失败: %w", err)
		}
	}

	// 监听ETCD配置变更
	c.watchETCD()

	return nil
}

// watchETCD 监听ETCD配置变更
func (c *Config[T]) watchETCD() {
	c.etcdClient.watch(func(data []byte) {
		// 检查配置是否已关闭
		c.closedMu.RLock()
		if c.closed {
			c.closedMu.RUnlock()
			return
		}
		c.closedMu.RUnlock()

		now := time.Now()
		// 防抖：如果与上次修改时间间隔小于设定的防抖时间，则忽略
		if now.Sub(c.lastModTime) < c.debounceTime {
			return
		}
		c.lastModTime = now

		// 保存旧配置
		c.oldData = cloneConfig(c.data)

		// 解析新配置
		var newData T
		if err := json.Unmarshal(data, &newData); err != nil {
			fmt.Printf("解析ETCD配置失败: %v\n", err)
			return
		}

		// 更新配置
		c.data = newData

		// 查找配置变更项
		changedItems := findConfigChanges(c.oldData, c.data, "")

		// 触发回调
		c.callbackMu.RLock()
		defer c.callbackMu.RUnlock()
		for _, callback := range c.changeCallbacks {
			if callback != nil {
				callback(fsnotify.Event{
					Name: c.etcdConfig.Key,
					Op:   fsnotify.Write,
				}, changedItems)
			}
		}
	})
}

// loadFromFile 从文件加载配置
func (c *Config[T]) loadFromFile() error {
	fileBytes, err := os.ReadFile(c.configFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 创建临时viper实例读取配置
	tempViper := viper.New()
	tempViper.SetConfigType(string(c.configType))

	// 从字节流读取配置
	if err := tempViper.ReadConfig(bytes.NewBuffer(fileBytes)); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 将读取的配置应用到当前的viper实例
	allSettings := tempViper.AllSettings()
	for k, val := range allSettings {
		c.v.Set(k, val)
	}

	// 将配置解析到结构体
	if err := c.v.Unmarshal(&c.data); err != nil {
		return fmt.Errorf("解析配置到结构体失败: %w", err)
	}

	return nil
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
	// 保存旧配置用于比较
	c.oldData = cloneConfig(c.data)

	// 更新配置
	c.data = data

	// 根据配置源保存
	if c.configFile != "" {
		return c.SaveConfig()
	} else if c.etcdClient != nil {
		return saveConfigToETCD(c.etcdClient, data)
	}

	return fmt.Errorf("未指定配置源")
}

// Close 关闭配置，停止监听并释放资源
func (c *Config[T]) Close() {
	// 设置关闭标志
	c.closedMu.Lock()
	c.closed = true
	c.closedMu.Unlock()

	// 清空回调函数列表
	c.callbackMu.Lock()
	c.changeCallbacks = nil
	c.callbackMu.Unlock()

	// 关闭ETCD客户端
	if c.etcdClient != nil {
		c.etcdClient.close()
		c.etcdClient = nil
	}

	// 释放其他资源
	c.v = nil
	c.data = *new(T)
	c.oldData = *new(T)
}
