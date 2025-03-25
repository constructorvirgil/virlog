package vconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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
	// 配置文件路径列表
	configFiles []string
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
	// ETCD配置列表
	etcdConfigs []*ETCDConfig
	// ETCD客户端列表
	etcdClients []*etcdClient
	// 是否仅使用环境变量
	envOnly bool
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

// 监听配置文件变更
func (c *Config[T]) watchConfig() {
	// 创建文件监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("创建文件监听器失败: %v\n", err)
		return
	}

	// 在后台运行监听
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					// 检查配置是否已关闭
					c.closedMu.RLock()
					if c.closed {
						c.closedMu.RUnlock()
						return
					}
					c.closedMu.RUnlock()

					// 等待文件写入完成
					time.Sleep(100 * time.Millisecond)

					// 重新加载配置
					if err := c.loadFromFile(); err != nil {
						fmt.Printf("配置文件变更后重新加载失败: %v\n", err)
						continue
					}

					// 触发回调
					c.triggerCallbacks(event)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("文件监听错误: %v\n", err)
			}
		}
	}()

	// 开始监听配置文件
	for _, configFile := range c.configFiles {
		if err := watcher.Add(configFile); err != nil {
			fmt.Printf("添加文件监听失败: %v\n", err)
		}
	}
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
	if len(config.configFiles) > 0 && len(config.etcdConfigs) > 0 {
		return nil, fmt.Errorf("不能同时使用配置文件和ETCD")
	}

	if len(config.configFiles) == 0 && len(config.etcdConfigs) == 0 && !config.envOnly { //默认使用环境变量
		config.envOnly = true
	}

	// 根据配置源初始化
	if config.envOnly {
		// 仅使用环境变量
		if err := config.initWithEnv(); err != nil {
			return nil, err
		}
	} else if len(config.configFiles) > 0 {
		// 使用配置文件
		if err := config.initWithFiles(); err != nil {
			return nil, err
		}
	} else {
		// 使用ETCD
		if err := config.initWithETCDs(); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// initWithFiles 使用配置文件初始化
func (c *Config[T]) initWithFiles() error {
	// 设置配置文件类型
	c.v.SetConfigType(string(c.configType))

	// 先将默认配置加载到viper中
	if err := c.bindStruct(c.data); err != nil {
		return fmt.Errorf("绑定默认配置失败: %w", err)
	}

	// 遍历所有配置文件
	for _, configFile := range c.configFiles {
		// 设置配置文件
		configDir := filepath.Dir(configFile)
		configName := filepath.Base(configFile)
		// 去掉扩展名
		ext := filepath.Ext(configName)
		if ext != "" {
			configName = configName[:len(configName)-len(ext)]
			// 如果没有指定配置类型，根据扩展名推断
			if c.configType == "" {
				switch strings.ToLower(ext[1:]) {
				case "json":
					c.configType = JSON
				case "yaml", "yml":
					c.configType = YAML
				case "toml":
					c.configType = TOML
				default:
					return fmt.Errorf("不支持的配置文件类型: %s", ext)
				}
				c.v.SetConfigType(string(c.configType))
			}
		}

		// 如果配置文件目录不存在，创建目录
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("创建配置目录失败: %w", err)
			}
		}

		// 创建临时viper实例
		tempViper := viper.New()
		tempViper.SetConfigType(string(c.configType))
		tempViper.AddConfigPath(configDir)
		tempViper.SetConfigName(configName)

		// 检查配置文件是否存在
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			// 如果配置文件不存在，则创建
			if err := tempViper.WriteConfigAs(configFile); err != nil {
				return fmt.Errorf("创建默认配置文件失败: %w", err)
			}
		}

		// 读取配置文件
		if err := tempViper.ReadInConfig(); err != nil {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}

		// 合并配置
		allSettings := tempViper.AllSettings()
		for k, v := range allSettings {
			c.v.Set(k, v)
		}
	}

	// 设置环境变量覆盖
	if c.enableEnv {
		// 获取所有配置键
		allKeys := c.v.AllKeys()
		for _, key := range allKeys {
			// 构造环境变量名
			envKey := fmt.Sprintf("%s_%s", c.envPrefix, strings.ToUpper(strings.ReplaceAll(key, ".", "_")))
			// 检查环境变量是否存在
			if envVal := os.Getenv(envKey); envVal != "" {
				// 根据配置值的类型进行转换
				switch c.v.Get(key).(type) {
				case int, int32, int64:
					if val, err := strconv.ParseInt(envVal, 10, 64); err == nil {
						c.v.Set(key, val)
					}
				case float32, float64:
					if val, err := strconv.ParseFloat(envVal, 64); err == nil {
						c.v.Set(key, val)
					}
				case bool:
					if val, err := strconv.ParseBool(envVal); err == nil {
						c.v.Set(key, val)
					}
				default:
					c.v.Set(key, envVal)
				}
			}
		}
	}

	// 将配置解析到结构体
	if err := c.v.Unmarshal(&c.data); err != nil {
		return fmt.Errorf("解析配置到结构体失败: %w", err)
	}

	// 监听配置文件变更
	c.watchConfig()

	return nil
}

// initWithETCDs 使用ETCD初始化
func (c *Config[T]) initWithETCDs() error {
	// 创建ETCD客户端
	clients := make([]*etcdClient, len(c.etcdConfigs))
	for i, etcdConfig := range c.etcdConfigs {
		client, err := newETCDClient(etcdConfig)
		if err != nil {
			return fmt.Errorf("创建ETCD客户端失败: %w", err)
		}
		clients[i] = client
	}
	c.etcdClients = clients

	// 从ETCD加载配置
	exists := true
	for i, client := range c.etcdClients {
		_, err := loadConfigFromETCD(client, &c.data, c.configType)
		if err != nil {
			fmt.Printf("从ETCD加载配置失败: %v, err=%v\n", c.etcdConfigs[i].Key, err)
			exists = false
		}
	}

	// 如果配置不存在，则保存默认配置到ETCD
	if !exists {
		for _, client := range c.etcdClients {
			err := saveConfigToETCD(client, c.data, c.configType)
			if err != nil {
				return fmt.Errorf("保存默认配置到ETCD失败: %w", err)
			}
		}
	}

	// 监听ETCD配置变更
	c.watchETCDs()

	return nil
}

// watchETCDs 监听ETCD配置变更
func (c *Config[T]) watchETCDs() {
	for i, client := range c.etcdClients {
		client.watch(func(data []byte) {
			// 检查配置是否已关闭
			c.closedMu.RLock()
			if c.closed {
				c.closedMu.RUnlock()
				return
			}
			c.closedMu.RUnlock()

			// 保存旧配置
			c.oldData = cloneConfig(c.data)

			// 根据配置类型解析新配置
			var (
				newData T
				err     error
			)

			switch c.configType {
			case JSON:
				err = json.Unmarshal(data, &newData)
			case YAML:
				err = yaml.Unmarshal(data, &newData)
			case TOML:
				err = toml.Unmarshal(data, &newData)
			default: // 默认使用 YAML
				err = yaml.Unmarshal(data, &newData)
			}

			if err != nil {
				fmt.Printf("解析ETCD配置失败: configType=%s, data=%v, err=%v\n", c.configType, string(data), err)
				return
			}

			// 更新配置
			c.data = newData

			// 查找配置变更项
			changedItems := findConfigChanges(c.oldData, c.data, c.configFiles[i])

			// 触发回调
			c.callbackMu.RLock()
			defer c.callbackMu.RUnlock()
			for _, callback := range c.changeCallbacks {
				if callback != nil {
					callback(fsnotify.Event{
						Name: c.etcdConfigs[i].Key,
						Op:   fsnotify.Write,
					}, changedItems)
				}
			}
		})
	}
}

// loadFromFile 从文件加载配置
func (c *Config[T]) loadFromFile() error {
	fileBytes, err := os.ReadFile(c.configFiles[0])
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
	// 根据配置类型选择正确的序列化方式
	var (
		configBytes []byte
		err         error
	)

	switch c.configType {
	case YAML:
		configBytes, err = yaml.Marshal(data)
	case JSON:
		configBytes, err = json.Marshal(data)
	case TOML:
		var buf bytes.Buffer
		err = toml.NewEncoder(&buf).Encode(data)
		configBytes = buf.Bytes()
	default:
		return fmt.Errorf("不支持的配置类型: %s", c.configType)
	}

	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 创建临时的 viper 实例
	tempViper := viper.New()
	tempViper.SetConfigType(string(c.configType))

	// 从序列化数据读取
	if err := tempViper.ReadConfig(bytes.NewBuffer(configBytes)); err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
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

	// 根据配置类型选择正确的写入方式
	var err error
	switch c.configType {
	case YAML:
		err = c.v.WriteConfigAs(c.configFiles[0])
	case JSON:
		jsonBytes, e := json.MarshalIndent(c.data, "", "  ")
		if e != nil {
			return fmt.Errorf("序列化JSON失败: %w", e)
		}
		err = os.WriteFile(c.configFiles[0], jsonBytes, 0644)
	case TOML:
		// 使用专门的TOML编码器
		var buf bytes.Buffer
		err = toml.NewEncoder(&buf).Encode(c.data)
		err = os.WriteFile(c.configFiles[0], buf.Bytes(), 0644)
	default:
		err = fmt.Errorf("不支持的配置类型: %s", c.configType)
	}

	if err != nil {
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
	// 根据配置源保存
	if len(c.configFiles) > 0 {
		return c.SaveConfig()
	} else if len(c.etcdClients) > 0 {
		return saveConfigToETCDs(c.etcdClients, data, c.configType)
	} else if c.envOnly {
		// 保存旧配置用于比较
		c.oldData = cloneConfig(c.data)

		// 更新内存中的配置数据
		c.data = data

		// 仅环境变量模式，只更新内存中的数据，不持久化
		// 重新绑定结构体到viper
		if err := c.bindStruct(c.data); err != nil {
			return fmt.Errorf("绑定结构体到配置失败: %w", err)
		}

		// 触发配置变更回调
		c.triggerCallbacks(fsnotify.Event{
			Name: "env",
			Op:   fsnotify.Write,
		})

		return nil
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
	for _, client := range c.etcdClients {
		client.close()
	}
	c.etcdClients = nil

	// 释放其他资源
	c.v = nil
	c.data = *new(T)
	c.oldData = *new(T)
}

// initWithEnv 仅使用环境变量初始化
func (c *Config[T]) initWithEnv() error {
	// 设置配置文件类型（用于序列化/反序列化）
	c.v.SetConfigType(string(c.configType))

	// 先将默认配置加载到viper中
	if err := c.bindStruct(c.data); err != nil {
		return fmt.Errorf("绑定默认配置失败: %w", err)
	}

	// 激活环境变量支持
	c.v.SetEnvPrefix(c.envPrefix)
	c.v.AutomaticEnv()
	c.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 绑定所有键到环境变量
	allKeys := c.v.AllKeys()
	for _, key := range allKeys {
		// 构造环境变量名
		envKey := fmt.Sprintf("%s_%s", c.envPrefix, strings.ToUpper(strings.ReplaceAll(key, ".", "_")))
		// 绑定环境变量
		if err := c.v.BindEnv(key, envKey); err != nil {
			return fmt.Errorf("绑定环境变量失败: %w", err)
		}

		// 检查环境变量是否存在并设置
		if envVal := os.Getenv(envKey); envVal != "" {
			// 根据配置值的类型进行转换
			switch c.v.Get(key).(type) {
			case int, int32, int64:
				if val, err := strconv.ParseInt(envVal, 10, 64); err == nil {
					c.v.Set(key, val)
				}
			case float32, float64:
				if val, err := strconv.ParseFloat(envVal, 64); err == nil {
					c.v.Set(key, val)
				}
			case bool:
				if val, err := strconv.ParseBool(envVal); err == nil {
					c.v.Set(key, val)
				}
			default:
				c.v.Set(key, envVal)
			}
		}
	}

	// 将配置解析到结构体
	if err := c.v.Unmarshal(&c.data); err != nil {
		return fmt.Errorf("解析配置到结构体失败: %w", err)
	}

	return nil
}

// saveConfigToETCDs 保存配置到多个ETCD
func saveConfigToETCDs(clients []*etcdClient, data interface{}, configType ConfigType) error {
	for _, client := range clients {
		if err := saveConfigToETCD(client, data, configType); err != nil {
			return err
		}
	}
	return nil
}
