package vconfig

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ETCDConfig ETCD配置
type ETCDConfig struct {
	// ETCD连接地址列表
	Endpoints []string
	// 连接超时时间
	DialTimeout time.Duration
	// 配置在ETCD中的key
	Key string
	// 用户名
	Username string
	// 密码
	Password string
	// TLS配置
	TLS *TLSConfig
}

// TLSConfig TLS配置
type TLSConfig struct {
	CertFile      string
	KeyFile       string
	TrustedCAFile string
}

// DefaultETCDConfig 返回默认的ETCD配置
func DefaultETCDConfig() *ETCDConfig {
	return &ETCDConfig{
		Endpoints:   []string{"192.168.33.10:2379"},
		DialTimeout: 5 * time.Second,
		Key:         "/config/app",
	}
}

// etcdClient ETCD客户端封装
type etcdClient struct {
	client *clientv3.Client
	config *ETCDConfig
	ctx    context.Context
	cancel context.CancelFunc
}

// newETCDClient 创建ETCD客户端
func newETCDClient(config *ETCDConfig) (*etcdClient, error) {
	// 创建context
	ctx, cancel := context.WithCancel(context.Background())

	// 构建ETCD客户端配置
	clientConfig := clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: config.DialTimeout,
		Username:    config.Username,
		Password:    config.Password,
	}

	// 如果配置了TLS
	if config.TLS != nil {
		tlsConfig, err := loadTLSConfig(config.TLS)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("加载TLS配置失败: %w", err)
		}
		clientConfig.TLS = tlsConfig
	}

	// 创建ETCD客户端
	client, err := clientv3.New(clientConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建ETCD客户端失败: %w", err)
	}

	return &etcdClient{
		client: client,
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// close 关闭ETCD客户端
func (e *etcdClient) close() error {
	e.cancel()
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

// get 从ETCD获取配置
func (e *etcdClient) get() ([]byte, error) {
	resp, err := e.client.Get(e.ctx, e.config.Key)
	if err != nil {
		return nil, fmt.Errorf("从ETCD获取配置失败: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	return resp.Kvs[0].Value, nil
}

// put 将配置保存到ETCD
func (e *etcdClient) put(data []byte) error {
	_, err := e.client.Put(e.ctx, e.config.Key, string(data))
	if err != nil {
		return fmt.Errorf("保存配置到ETCD失败: %w", err)
	}
	return nil
}

// watch 监听ETCD配置变更
func (e *etcdClient) watch(callback func([]byte)) {
	watchChan := e.client.Watch(e.ctx, e.config.Key)
	go func() {
		for resp := range watchChan {
			for _, ev := range resp.Events {
				if ev.Type == clientv3.EventTypePut {
					callback(ev.Kv.Value)
				}
			}
		}
	}()
}

// loadTLSConfig 加载TLS配置
func loadTLSConfig(config *TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("加载证书失败: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// saveConfigToETCD 保存配置到ETCD
func saveConfigToETCD[T any](client *etcdClient, data T) error {
	// 将配置序列化为JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 保存到ETCD
	return client.put(jsonData)
}

// loadConfigFromETCD 从ETCD加载配置
func loadConfigFromETCD[T any](client *etcdClient, data *T) error {
	// 从ETCD获取配置
	jsonData, err := client.get()
	if err != nil {
		return fmt.Errorf("从ETCD获取配置失败: %w", err)
	}

	// 如果配置不存在，返回nil
	if jsonData == nil {
		return nil
	}

	// 反序列化配置
	if err := json.Unmarshal(jsonData, data); err != nil {
		return fmt.Errorf("反序列化配置失败: %w", err)
	}

	return nil
}
