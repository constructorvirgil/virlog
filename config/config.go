package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	// 默认环境变量前缀
	DefaultEnvPrefix = "VIRLOG_"
	// 用于指定配置文件路径的环境变量
	EnvConfigFile = "VIRLOG_CONFFILE"
	// 用于指定自定义环境变量前缀的环境变量
	EnvPrefix = "VIRLOG_PREFIX"
)

// 全局配置管理器
var (
	// 全局viper实例
	v *viper.Viper
	// 全局配置
	globalConfig *Config
	// 环境变量前缀
	envPrefix string
	// 监听器列表
	listeners []chan<- *Config
	// 监听器锁
	listenerMutex sync.Mutex
	// 配置文件路径
	configFile string
	// 初始化只执行一次
	initOnce sync.Once
)

// Config 包含日志配置选项
type Config struct {
	// 日志级别
	Level string `json:"level" yaml:"level" mapstructure:"level"`
	// 日志格式 "json" 或 "console"
	Format string `json:"format" yaml:"format" mapstructure:"format"`
	// 输出位置，支持 "stdout", "stderr", "file"
	Output string `json:"output" yaml:"output" mapstructure:"output"`
	// 文件输出配置
	FileConfig *FileConfig `json:"file_config" yaml:"file_config" mapstructure:"file_config"`
	// 开发模式
	Development bool `json:"development" yaml:"development" mapstructure:"development"`
	// 是否添加调用者信息
	EnableCaller bool `json:"enable_caller" yaml:"enable_caller" mapstructure:"enable_caller"`
	// 调用栈
	EnableStacktrace bool `json:"enable_stacktrace" yaml:"enable_stacktrace" mapstructure:"enable_stacktrace"`
	// 采样配置
	EnableSampling bool `json:"enable_sampling" yaml:"enable_sampling" mapstructure:"enable_sampling"`
	// 日志字段配置
	DefaultFields map[string]interface{} `json:"default_fields" yaml:"default_fields" mapstructure:"default_fields"`
}

// FileConfig 包含文件输出的配置
type FileConfig struct {
	// 日志文件路径
	Filename string `json:"filename" yaml:"filename" mapstructure:"filename"`
	// 单个日志文件的最大大小（MB）
	MaxSize int `json:"max_size" yaml:"max_size" mapstructure:"max_size"`
	// 保留的旧日志文件的最大数量
	MaxBackups int `json:"max_backups" yaml:"max_backups" mapstructure:"max_backups"`
	// 保留日志文件的最大天数
	MaxAge int `json:"max_age" yaml:"max_age" mapstructure:"max_age"`
	// 是否压缩旧日志
	Compress bool `json:"compress" yaml:"compress" mapstructure:"compress"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:            "info",
		Format:           "json",
		Output:           "stdout",
		Development:      false,
		EnableCaller:     true,
		EnableStacktrace: true,
		EnableSampling:   false,
		DefaultFields:    make(map[string]interface{}),
		FileConfig: &FileConfig{
			Filename:   "./logs/app.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
	}
}

// 初始化配置管理器
func initConfig() {
	initOnce.Do(func() {
		// 设置环境变量前缀
		prefix := os.Getenv(EnvPrefix)
		if prefix == "" {
			envPrefix = DefaultEnvPrefix
		} else {
			envPrefix = prefix
		}

		// 初始化全局配置
		globalConfig = DefaultConfig()

		// 创建viper实例
		v = viper.New()

		// 检查是否指定了配置文件
		configFile = os.Getenv(EnvConfigFile)
		if configFile != "" {
			loadConfigFile(configFile)
		}

		// 加载环境变量配置
		loadEnvConfig()

		// 监听配置文件变化
		if configFile != "" {
			v.WatchConfig()
			v.OnConfigChange(func(e fsnotify.Event) {
				// 配置文件发生变化，重新加载
				fmt.Printf("配置文件已变更: %s\n", e.Name)

				// 重新加载配置文件
				if err := v.ReadInConfig(); err != nil {
					fmt.Printf("读取配置文件失败: %v\n", err)
					return
				}

				// 更新全局配置
				newConfig := DefaultConfig()
				if err := v.Unmarshal(newConfig); err != nil {
					fmt.Printf("解析配置失败: %v\n", err)
					return
				}

				// 环境变量优先级高于配置文件
				overrideWithEnv(newConfig)

				// 更新全局配置
				globalConfig = newConfig

				// 通知监听器
				notifyListeners(newConfig)
			})
		}
	})
}

// 加载配置文件
func loadConfigFile(filePath string) {
	// 设置配置文件路径
	v.SetConfigFile(filePath)

	// 设置配置类型
	configType := getConfigType(filePath)
	v.SetConfigType(configType)

	// 尝试读取配置文件
	if err := v.ReadInConfig(); err != nil {
		fmt.Printf("读取配置文件失败，使用默认配置: %v\n", err)
		return
	}

	// 解析配置
	if err := v.Unmarshal(globalConfig); err != nil {
		fmt.Printf("解析配置失败，使用默认配置: %v\n", err)
		globalConfig = DefaultConfig()
	}
}

// 加载环境变量配置
func loadEnvConfig() {
	// 将环境变量绑定到配置
	overrideWithEnv(globalConfig)
}

// 使用环境变量覆盖配置
func overrideWithEnv(cfg *Config) {
	// 日志级别
	if level := getEnv("LEVEL"); level != "" {
		cfg.Level = level
	}

	// 日志格式
	if format := getEnv("FORMAT"); format != "" {
		cfg.Format = format
	}

	// 输出位置
	if output := getEnv("OUTPUT"); output != "" {
		cfg.Output = output
	}

	// 开发模式
	if dev := getEnv("DEVELOPMENT"); dev == "true" {
		cfg.Development = true
	} else if dev == "false" {
		cfg.Development = false
	}

	// 调用者信息
	if caller := getEnv("ENABLE_CALLER"); caller == "true" {
		cfg.EnableCaller = true
	} else if caller == "false" {
		cfg.EnableCaller = false
	}

	// 调用栈
	if stacktrace := getEnv("ENABLE_STACKTRACE"); stacktrace == "true" {
		cfg.EnableStacktrace = true
	} else if stacktrace == "false" {
		cfg.EnableStacktrace = false
	}

	// 采样
	if sampling := getEnv("ENABLE_SAMPLING"); sampling == "true" {
		cfg.EnableSampling = true
	} else if sampling == "false" {
		cfg.EnableSampling = false
	}

	// 文件配置
	if filename := getEnv("FILE_PATH"); filename != "" {
		cfg.FileConfig.Filename = filename
	}

	if maxSize := getEnv("FILE_MAX_SIZE"); maxSize != "" {
		if size, err := parseInt(maxSize); err == nil && size > 0 {
			cfg.FileConfig.MaxSize = size
		}
	}

	if maxBackups := getEnv("FILE_MAX_BACKUPS"); maxBackups != "" {
		if backups, err := parseInt(maxBackups); err == nil && backups >= 0 {
			cfg.FileConfig.MaxBackups = backups
		}
	}

	if maxAge := getEnv("FILE_MAX_AGE"); maxAge != "" {
		if age, err := parseInt(maxAge); err == nil && age >= 0 {
			cfg.FileConfig.MaxAge = age
		}
	}

	if compress := getEnv("FILE_COMPRESS"); compress == "true" {
		cfg.FileConfig.Compress = true
	} else if compress == "false" {
		cfg.FileConfig.Compress = false
	}
}

// 从环境变量中获取配置
func getEnv(key string) string {
	return os.Getenv(envPrefix + key)
}

// 将字符串转为整数
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// 添加配置变更监听器
func AddListener(listener chan<- *Config) {
	listenerMutex.Lock()
	defer listenerMutex.Unlock()

	listeners = append(listeners, listener)
	// 立即发送当前配置
	listener <- GetConfig()
}

// 移除配置变更监听器
func RemoveListener(listener chan<- *Config) {
	listenerMutex.Lock()
	defer listenerMutex.Unlock()

	for i, l := range listeners {
		if l == listener {
			listeners = append(listeners[:i], listeners[i+1:]...)
			return
		}
	}
}

// 通知所有监听器配置已变更
func notifyListeners(cfg *Config) {
	listenerMutex.Lock()
	defer listenerMutex.Unlock()

	for _, listener := range listeners {
		select {
		case listener <- cfg:
			// 发送成功
		case <-time.After(100 * time.Millisecond):
			// 超时，监听器可能已被阻塞，跳过
			fmt.Println("监听器接收超时")
		}
	}
}

// LoadFromFile 从文件加载日志配置
func LoadFromFile(filePath string) (*Config, error) {
	ext := filepath.Ext(filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := DefaultConfig()
	switch ext {
	case ".json":
		if err := json.Unmarshal(content, config); err != nil {
			return nil, fmt.Errorf("解析JSON配置失败: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, config); err != nil {
			return nil, fmt.Errorf("解析YAML配置失败: %w", err)
		}
	default:
		return nil, fmt.Errorf("不支持的配置文件格式: %s", ext)
	}

	return config, nil
}

// SaveToFile 将配置保存到文件
func SaveToFile(config *Config, filePath string) error {
	if config == nil {
		config = DefaultConfig()
	}

	var (
		content []byte
		err     error
	)

	ext := filepath.Ext(filePath)
	switch ext {
	case ".json":
		content, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化JSON配置失败: %w", err)
		}
	case ".yaml", ".yml":
		content, err = yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("序列化YAML配置失败: %w", err)
		}
	default:
		return fmt.Errorf("不支持的配置文件格式: %s", ext)
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// FromEnv 从环境变量加载配置（兼容旧函数）
func FromEnv() *Config {
	initConfig()
	return GetConfig()
}

// GetConfig 获取当前配置
func GetConfig() *Config {
	initConfig()

	// 返回深拷贝，避免外部修改影响内部配置
	configCopy := *globalConfig
	fileConfigCopy := *globalConfig.FileConfig
	configCopy.FileConfig = &fileConfigCopy

	// 拷贝默认字段
	defaultFields := make(map[string]interface{})
	for k, v := range globalConfig.DefaultFields {
		defaultFields[k] = v
	}
	configCopy.DefaultFields = defaultFields

	return &configCopy
}

// SetConfig 设置配置（仅用于测试）
func SetConfig(cfg *Config) {
	if cfg == nil {
		globalConfig = DefaultConfig()
	} else {
		globalConfig = cfg
	}

	// 通知所有监听器
	notifyListeners(globalConfig)
}

// GetEnvPrefix 获取当前环境变量前缀
func GetEnvPrefix() string {
	initConfig()
	return envPrefix
}

// 文件扩展名转文件类型
func getConfigType(filePath string) string {
	ext := filepath.Ext(filePath)

	ext = strings.TrimPrefix(ext, ".")

	if ext == "" {
		// 如果没有扩展名，尝试根据文件名判断
		if strings.HasSuffix(filePath, "json") {
			return "json"
		} else if strings.HasSuffix(filePath, "yaml") || strings.HasSuffix(filePath, "yml") {
			return "yaml"
		}
		// 默认使用json
		return "json"
	}

	switch ext {
	case "yml":
		return "yaml"
	default:
		return ext
	}
}
