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

// WithETCDConfig 设置ETCD配置
func WithETCDConfig[T any](config *ETCDConfig) ConfigOption[T] {
	return func(c *Config[T]) {
		c.etcdConfig = config
	}
}

// WithETCDEndpoints 设置ETCD连接地址
func WithETCDEndpoints[T any](endpoints ...string) ConfigOption[T] {
	return func(c *Config[T]) {
		if c.etcdConfig == nil {
			c.etcdConfig = DefaultETCDConfig()
		}
		c.etcdConfig.Endpoints = endpoints
	}
}

// WithETCDAuth 设置ETCD认证信息
func WithETCDAuth[T any](username, password string) ConfigOption[T] {
	return func(c *Config[T]) {
		if c.etcdConfig == nil {
			c.etcdConfig = DefaultETCDConfig()
		}
		c.etcdConfig.Username = username
		c.etcdConfig.Password = password
	}
}

// WithETCDKey 设置ETCD中的配置key
func WithETCDKey[T any](key string) ConfigOption[T] {
	return func(c *Config[T]) {
		if c.etcdConfig == nil {
			c.etcdConfig = DefaultETCDConfig()
		}
		c.etcdConfig.Key = key
	}
}

// WithETCDTLS 设置ETCD的TLS配置
func WithETCDTLS[T any](certFile, keyFile, caFile string) ConfigOption[T] {
	return func(c *Config[T]) {
		if c.etcdConfig == nil {
			c.etcdConfig = DefaultETCDConfig()
		}
		c.etcdConfig.TLS = &TLSConfig{
			CertFile:      certFile,
			KeyFile:       keyFile,
			TrustedCAFile: caFile,
		}
	}
}

// WithEnvOnly 仅使用环境变量配置，不需要配置文件或ETCD
func WithEnvOnly[T any](prefix string) ConfigOption[T] {
	return func(c *Config[T]) {
		c.enableEnv = true
		c.envPrefix = prefix
		c.envOnly = true
	}
}
