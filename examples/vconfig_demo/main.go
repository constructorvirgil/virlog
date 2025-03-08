package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/virlog/vconfig"
)

// 应用配置结构体
type AppConfig struct {
	App struct {
		Name        string `yaml:"name" json:"name"`
		Version     string `yaml:"version" json:"version"`
		Environment string `yaml:"environment" json:"environment"`
	} `yaml:"app" json:"app"`
	HTTP struct {
		Host           string        `yaml:"host" json:"host"`
		Port           int           `yaml:"port" json:"port"`
		ReadTimeout    time.Duration `yaml:"read_timeout" json:"read_timeout"`
		WriteTimeout   time.Duration `yaml:"write_timeout" json:"write_timeout"`
		MaxHeaderBytes int           `yaml:"max_header_bytes" json:"max_header_bytes"`
	} `yaml:"http" json:"http"`
	Log struct {
		Level      string `yaml:"level" json:"level"`
		Format     string `yaml:"format" json:"format"`
		OutputPath string `yaml:"output_path" json:"output_path"`
	} `yaml:"log" json:"log"`
}

// 创建默认配置
func newDefaultConfig() AppConfig {
	config := AppConfig{}

	// 应用基本信息
	config.App.Name = "配置示例应用"
	config.App.Version = "1.0.0"
	config.App.Environment = "development"

	// HTTP服务器配置
	config.HTTP.Host = "localhost"
	config.HTTP.Port = 8080
	config.HTTP.ReadTimeout = 10 * time.Second
	config.HTTP.WriteTimeout = 10 * time.Second
	config.HTTP.MaxHeaderBytes = 1 << 20 // 1MB

	// 日志配置
	config.Log.Level = "info"
	config.Log.Format = "json"
	config.Log.OutputPath = "logs/app.log"

	return config
}

// HTTP服务器结构体
type Server struct {
	server *http.Server
	config *vconfig.Config[AppConfig]
}

// 创建并配置HTTP服务器
func NewServer(config *vconfig.Config[AppConfig]) *Server {
	cfg := config.GetData()

	// 创建路由
	mux := http.NewServeMux()

	// 注册路由处理函数
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "欢迎访问 %s (版本: %s)\n", cfg.App.Name, cfg.App.Version)
		fmt.Fprintf(w, "环境: %s\n", cfg.App.Environment)
		fmt.Fprintf(w, "当前时间: %s\n", time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		// 获取最新配置
		currentCfg := config.GetData()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "当前配置:\n")
		fmt.Fprintf(w, "应用名称: %s\n", currentCfg.App.Name)
		fmt.Fprintf(w, "版本: %s\n", currentCfg.App.Version)
		fmt.Fprintf(w, "环境: %s\n", currentCfg.App.Environment)
		fmt.Fprintf(w, "HTTP端口: %d\n", currentCfg.HTTP.Port)
		fmt.Fprintf(w, "日志级别: %s\n", currentCfg.Log.Level)
	})

	// 创建HTTP服务器
	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	httpServer := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    cfg.HTTP.ReadTimeout,
		WriteTimeout:   cfg.HTTP.WriteTimeout,
		MaxHeaderBytes: cfg.HTTP.MaxHeaderBytes,
	}

	// 监听配置变更
	config.OnChange(func(e fsnotify.Event, changedItems []vconfig.ConfigChangedItem) {
		log.Printf("配置已更新，重新加载服务器配置")

		// 打印变动的配置项
		if len(changedItems) > 0 {
			log.Printf("发现 %d 个配置变更:", len(changedItems))
			for _, item := range changedItems {
				log.Printf("  - 配置项: %s, 旧值: %v, 新值: %v", item.Path, item.OldValue, item.NewValue)
			}
		}
	})

	return &Server{
		server: httpServer,
		config: config,
	}
}

// 启动HTTP服务器
func (s *Server) Start() {
	cfg := s.config.GetData()
	log.Printf("启动HTTP服务器在 %s:%d", cfg.HTTP.Host, cfg.HTTP.Port)

	// 非阻塞启动服务器
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP服务器错误: %v", err)
		}
	}()
}

// 关闭HTTP服务器
func (s *Server) Shutdown() {
	log.Println("正在关闭HTTP服务器...")

	// 创建一个5秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("服务器关闭错误: %v", err)
	}

	log.Println("HTTP服务器已关闭")
}

func main() {
	// 确保配置目录存在
	configDir := "configs"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatalf("创建配置目录失败: %v", err)
	}

	configFile := filepath.Join(configDir, "app.yaml")

	// 创建配置管理器
	cfg, err := vconfig.NewConfig(newDefaultConfig(),
		vconfig.WithConfigFile[AppConfig](configFile),
		vconfig.WithConfigType[AppConfig](vconfig.YAML),
		vconfig.WithEnvPrefix[AppConfig]("APP"),
		vconfig.WithDebounceTime[AppConfig](1*time.Second))

	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	// 打印初始配置
	appConfig := cfg.GetData()
	log.Printf("加载配置成功: %s v%s", appConfig.App.Name, appConfig.App.Version)
	log.Printf("配置文件路径: %s", configFile)
	log.Printf("HTTP服务器将在 %s:%d 上运行", appConfig.HTTP.Host, appConfig.HTTP.Port)

	// 创建并启动HTTP服务器
	server := NewServer(cfg)
	server.Start()

	// 设置优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("收到关闭信号，开始优雅关闭...")
	server.Shutdown()
	log.Println("程序已退出")
}
