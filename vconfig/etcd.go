package vconfig

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

// ETCDConfig ETCD配置
type ETCDConfig struct {
	// ETCD服务器地址列表
	Endpoints []string
	// 用户名
	Username string
	// 密码
	Password string
	// 配置键
	Key string
	// 超时时间
	Timeout time.Duration
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
		Endpoints: []string{"etcd-test:2379"},
		Timeout:   5 * time.Second,
		Key:       "/config/app",
	}
}

// etcdClient ETCD客户端
type etcdClient struct {
	// 客户端
	client *clientv3.Client
	// 配置键
	key string
	// 是否已关闭
	closed bool
	// 保护closed字段的互斥锁
	closedMu sync.RWMutex
}

// newETCDClient 创建ETCD客户端
func newETCDClient(config *ETCDConfig) (*etcdClient, error) {
	// 创建客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Endpoints,
		Username:    config.Username,
		Password:    config.Password,
		DialTimeout: config.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("创建ETCD客户端失败: %w", err)
	}

	return &etcdClient{
		client: client,
		key:    config.Key,
	}, nil
}

// loadConfigFromETCD 从ETCD加载配置
func loadConfigFromETCD(client *etcdClient, data interface{}, configType ConfigType) (bool, error) {
	// 获取配置
	resp, err := client.client.Get(context.Background(), client.key)
	if err != nil {
		return false, fmt.Errorf("获取ETCD配置失败: %w", err)
	}

	// 如果配置不存在
	if len(resp.Kvs) == 0 {
		return false, nil
	}

	// 根据配置类型解析
	switch configType {
	case JSON:
		if err := json.Unmarshal(resp.Kvs[0].Value, data); err != nil {
			return false, fmt.Errorf("解析JSON配置失败: %w", err)
		}
	case YAML:
		if err := yaml.Unmarshal(resp.Kvs[0].Value, data); err != nil {
			return false, fmt.Errorf("解析YAML配置失败: %w", err)
		}
	case TOML:
		if err := toml.Unmarshal(resp.Kvs[0].Value, data); err != nil {
			return false, fmt.Errorf("解析TOML配置失败: %w", err)
		}
	default:
		return false, fmt.Errorf("不支持的配置类型: %s", configType)
	}

	return true, nil
}

// saveConfigToETCD 保存配置到ETCD
func saveConfigToETCD(client *etcdClient, data interface{}, configType ConfigType) error {
	// 根据配置类型序列化
	var configBytes []byte
	var err error

	switch configType {
	case JSON:
		configBytes, err = json.Marshal(data)
	case YAML:
		configBytes, err = yaml.Marshal(data)
	case TOML:
		var buf bytes.Buffer
		err = toml.NewEncoder(&buf).Encode(data)
		configBytes = buf.Bytes()
	default:
		return fmt.Errorf("不支持的配置类型: %s", configType)
	}

	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 保存配置
	_, err = client.client.Put(context.Background(), client.key, string(configBytes))
	if err != nil {
		return fmt.Errorf("保存ETCD配置失败: %w", err)
	}

	return nil
}

// watch 监听配置变更
func (c *etcdClient) watch(callback func([]byte)) {
	// 创建监听器
	watcher := c.client.Watch(context.Background(), c.key)

	// 在后台运行监听
	go func() {
		for {
			select {
			case resp, ok := <-watcher:
				if !ok {
					return
				}

				// 检查客户端是否已关闭
				c.closedMu.RLock()
				if c.closed {
					c.closedMu.RUnlock()
					return
				}
				c.closedMu.RUnlock()

				// 处理事件
				for _, ev := range resp.Events {
					if ev.Type == clientv3.EventTypePut {
						callback(ev.Kv.Value)
					}
				}
			}
		}
	}()
}

// close 关闭客户端
func (c *etcdClient) close() {
	// 设置关闭标志
	c.closedMu.Lock()
	c.closed = true
	c.closedMu.Unlock()

	// 关闭客户端
	if c.client != nil {
		c.client.Close()
	}
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
