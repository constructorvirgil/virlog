package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/virlog/config"
	"github.com/virlog/logger"
)

func main() {
	// 创建日志配置
	cfg := config.DefaultConfig()
	cfg.Level = "debug"
	cfg.Format = "json"
	cfg.DefaultFields = map[string]interface{}{
		"service": "api-server",
		"version": "1.0.0",
	}

	// 创建日志实例
	log, err := logger.NewLogger(cfg)
	if err != nil {
		logger.Fatal("创建日志器失败", logger.Err(err))
	}
	defer log.Sync()

	// 设置为默认日志
	logger.SetDefault(log)

	// 创建HTTP路由
	mux := http.NewServeMux()

	// 注册路由处理器
	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/api/users", usersHandler)
	mux.HandleFunc("/api/error", errorHandler)

	// 应用日志中间件
	loggedHandler := logger.HTTPMiddleware(log)(mux)

	// 启动一个协程，每隔3秒请求一次服务接口
	go func() {
		// 等待服务器启动
		time.Sleep(1 * time.Second)

		endpoints := []string{
			"http://localhost:8080/",
			"http://localhost:8080/api/users",
			"http://localhost:8080/api/error",
		}

		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		// 无限循环请求
		for {
			for _, endpoint := range endpoints {
				// 请求服务接口
				resp, err := client.Get(endpoint)
				if err != nil {
					fmt.Printf("请求 %s 失败: %v\n", endpoint, err)
					continue
				}

				// 关闭响应体
				resp.Body.Close()

				// 等待1秒再发下一个请求
				time.Sleep(1 * time.Second)
			}

			// 等待5秒再发起下一轮请求
			time.Sleep(5 * time.Second)
		}
	}()

	// 启动HTTP服务
	logger.Info("HTTP服务启动", logger.String("addr", ":8080"))
	if err := http.ListenAndServe(":8080", loggedHandler); err != nil {
		logger.Fatal("HTTP服务启动失败", logger.Err(err))
	}
}

// 首页处理器
func homeHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLoggerFromContext(r.Context())
	log.Debug("处理首页请求")

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("欢迎访问API服务"))
}

// 用户API处理器
func usersHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLoggerFromContext(r.Context())
	log.Debug("处理用户API请求")

	users := []map[string]interface{}{
		{"id": 1, "name": "张三", "email": "zhangsan@example.com"},
		{"id": 2, "name": "李四", "email": "lisi@example.com"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)

	log.Info("返回用户列表", logger.Int("count", len(users)))
}

// 错误处理器
func errorHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLoggerFromContext(r.Context())
	log.Debug("处理错误演示API")

	err := &apiError{
		Code:    500,
		Message: "内部服务错误",
	}

	log.Error("处理请求时出错",
		logger.Int("code", err.Code),
		logger.String("message", err.Message),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)
	json.NewEncoder(w).Encode(err)
}

// API错误类型
type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
