package vconfig

import (
	"time"
)

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
