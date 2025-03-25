package vconfig

import (
	"time"
)

// ConfigOption 配置选项函数类型
type ConfigOption[T any] func(*Config[T])

// WithConfigFile 设置配置文件路径
func WithConfigFile[T any](configFile string) ConfigOption[T] {
	return func(c *Config[T]) {
		c.configFiles = []string{configFile}
	}
}

// WithConfigFiles 设置多个配置文件路径
func WithConfigFiles[T any](configFiles []string) ConfigOption[T] {
	return func(c *Config[T]) {
		c.configFiles = configFiles
	}
}

// WithConfigType 设置配置文件类型
func WithConfigType[T any](configType ConfigType) ConfigOption[T] {
	return func(c *Config[T]) {
		c.configType = configType
	}
}

// WithEnv 启用环境变量支持
func WithEnv[T any](enable bool) ConfigOption[T] {
	return func(c *Config[T]) {
		c.enableEnv = enable
	}
}

// WithEnvPrefix 设置环境变量前缀
func WithEnvPrefix[T any](prefix string) ConfigOption[T] {
	return func(c *Config[T]) {
		c.envPrefix = prefix
	}
}

// WithETCD 设置ETCD配置
func WithETCD[T any](etcdConfig *ETCDConfig) ConfigOption[T] {
	return func(c *Config[T]) {
		c.etcdConfigs = []*ETCDConfig{etcdConfig}
	}
}

// WithETCDs 设置多个ETCD配置
func WithETCDs[T any](etcdConfigs []*ETCDConfig) ConfigOption[T] {
	return func(c *Config[T]) {
		c.etcdConfigs = etcdConfigs
	}
}

// WithEnvOnly 设置是否仅使用环境变量
func WithEnvOnly[T any](envOnly bool) ConfigOption[T] {
	return func(c *Config[T]) {
		c.envOnly = envOnly
	}
}

// WithDebounceTime 设置防抖时间
func WithDebounceTime[T any](debounceTime time.Duration) ConfigOption[T] {
	return func(c *Config[T]) {
		c.debounceTime = debounceTime
	}
}

// WithETCDEndpoints 设置ETCD连接地址
func WithETCDEndpoints[T any](endpoints ...string) ConfigOption[T] {
	return func(c *Config[T]) {
		if len(c.etcdConfigs) == 0 {
			c.etcdConfigs = []*ETCDConfig{DefaultETCDConfig()}
		}
		c.etcdConfigs[0].Endpoints = endpoints
	}
}

// WithETCDAuth 设置ETCD认证信息
func WithETCDAuth[T any](username, password string) ConfigOption[T] {
	return func(c *Config[T]) {
		if len(c.etcdConfigs) == 0 {
			c.etcdConfigs = []*ETCDConfig{DefaultETCDConfig()}
		}
		c.etcdConfigs[0].Username = username
		c.etcdConfigs[0].Password = password
	}
}

// WithETCDKey 设置ETCD中的配置key
func WithETCDKey[T any](key string) ConfigOption[T] {
	return func(c *Config[T]) {
		if len(c.etcdConfigs) == 0 {
			c.etcdConfigs = []*ETCDConfig{DefaultETCDConfig()}
		}
		c.etcdConfigs[0].Key = key
	}
}

// WithETCDTLS 设置ETCD的TLS配置
func WithETCDTLS[T any](certFile, keyFile, caFile string) ConfigOption[T] {
	return func(c *Config[T]) {
		if len(c.etcdConfigs) == 0 {
			c.etcdConfigs = []*ETCDConfig{DefaultETCDConfig()}
		}
		c.etcdConfigs[0].TLS = &TLSConfig{
			CertFile:      certFile,
			KeyFile:       keyFile,
			TrustedCAFile: caFile,
		}
	}
}
