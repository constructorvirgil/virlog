# virlog

[![GoDoc](https://godoc.org/github.com/constructorvirgil/virlog?status.svg)](https://godoc.org/github.com/constructorvirgil/virlog)
[![Go Report Card](https://goreportcard.com/badge/github.com/constructorvirgil/virlog)](https://goreportcard.com/report/github.com/constructorvirgil/virlog)

virlog 是一个基于 [zap](https://github.com/uber-go/zap) 的高性能、可扩展的结构化日志库，提供开箱即用的日志功能，适用于各种 Go 应用程序。

## 特性

- **高性能**：基于 zap 的高性能结构化日志
- **开箱即用**：提供默认配置，可以直接使用
- **灵活配置**：支持 JSON、YAML 配置文件和环境变量配置
- **实时配置**：支持配置文件热加载，动态更新日志器
- **自定义前缀**：支持自定义环境变量前缀
- **结构化日志**：支持字段化日志记录
- **级别控制**：动态调整日志级别
- **多种输出**：支持控制台、文件输出
- **文件轮转**：基于 lumberjack 的日志文件轮转
- **上下文支持**：支持从上下文中提取和注入日志器
- **HTTP 中间件**：内置 HTTP 服务日志中间件
- **完整测试**：100% 测试覆盖率

## 安装

```bash
go get github.com/constructorvirgil/virlog
```

## 快速开始

### 基础用法

```go
package main

import (
	"github.com/constructorvirgil/virlog/logger"
)

func main() {
	// 使用默认配置的全局日志器
	logger.Info("这是一条信息日志", logger.String("key", "value"))
	logger.Error("这是一条错误日志", logger.Int("code", 500))

	// 创建自定义日志器
	cfg := config.DefaultConfig()
	cfg.Level = "debug"
	cfg.Format = "console"
	cfg.DefaultFields = map[string]interface{}{
		"service": "my-service",
	}

	log, err := logger.NewLogger(cfg)
	if err != nil {
		logger.Fatal("创建日志器失败", logger.Err(err))
	}
	defer log.Sync()

	// 使用自定义日志器
	log.Debug("这是一条调试日志")
	log.Info("这是一条信息日志", logger.String("custom", "value"))

	// 带字段的logger
	userLogger := log.With(
		logger.String("user_id", "12345"),
		logger.String("username", "test_user"),
	)
	userLogger.Info("用户登录成功")
}
```

### 配置文件

virlog 支持从配置文件加载配置，并且支持配置热加载（自动监听配置文件变化）：

```go
package main

import (
	"os"

	"github.com/constructorvirgil/virlog/config"
	"github.com/constructorvirgil/virlog/logger"
)

func main() {
	// 设置配置文件路径
	os.Setenv("VIRLOG_CONFFILE", "config.json")

	// 配置会自动加载，并监听变化
	// 全局日志器会自动更新
	logger.Info("应用启动")

	// 使用当前配置
	cfg := config.GetConfig()
	logger.Info("当前配置",
		logger.String("level", cfg.Level),
		logger.String("format", cfg.Format))

	// 如果配置文件发生变化，日志器会自动更新
}
```

配置文件示例 (config.json):

```json
{
  "level": "debug",
  "format": "console",
  "output": "stdout",
  "development": true,
  "enable_caller": true,
  "enable_stacktrace": true,
  "default_fields": {
    "app": "my-app",
    "env": "development"
  },
  "file_config": {
    "filename": "./logs/app.log",
    "max_size": 100,
    "max_backups": 3,
    "max_age": 28,
    "compress": true
  }
}
```

### 自定义环境变量前缀

virlog 默认使用 `VIRLOG_` 作为环境变量的前缀，你可以通过设置 `VIRLOG_PREFIX` 环境变量来修改：

```bash
# 设置自定义前缀
export VIRLOG_PREFIX=MYAPP_

# 使用自定义前缀设置日志级别
export MYAPP_LEVEL=debug
```

```go
// 代码中会自动使用自定义前缀
logger.Info("当前环境变量前缀", logger.String("prefix", config.GetEnvPrefix()))
```

### HTTP 服务中间件

```go
package main

import (
	"net/http"

	"github.com/constructorvirgil/virlog/logger"
)

func main() {
	// 创建HTTP路由
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 从请求上下文获取日志器
		log := logger.GetLoggerFromContext(r.Context())
		log.Info("处理请求")

		w.Write([]byte("Hello, World!"))
	})

	// 应用日志中间件
	handler := logger.HTTPMiddleware(logger.DefaultLogger())(mux)

	// 启动HTTP服务
	http.ListenAndServe(":8080", handler)
}
```

## 配置选项

| 选项                  | 环境变量                 | 描述                                                       | 默认值         |
| --------------------- | ------------------------ | ---------------------------------------------------------- | -------------- |
| Level                 | VIRLOG_LEVEL             | 日志级别（debug, info, warn, error, dpanic, panic, fatal） | info           |
| Format                | VIRLOG_FORMAT            | 日志格式（json, console）                                  | json           |
| Output                | VIRLOG_OUTPUT            | 输出位置（stdout, stderr, file）                           | stdout         |
| Development           | VIRLOG_DEVELOPMENT       | 开发模式（彩色日志，完整调用者信息）                       | false          |
| EnableCaller          | VIRLOG_ENABLE_CALLER     | 是否记录调用者信息                                         | true           |
| EnableStacktrace      | VIRLOG_ENABLE_STACKTRACE | 是否记录错误栈信息                                         | true           |
| EnableSampling        | VIRLOG_ENABLE_SAMPLING   | 是否启用日志采样                                           | false          |
| DefaultFields         | -                        | 默认字段                                                   | {}             |
| FileConfig.Filename   | VIRLOG_FILE_PATH         | 日志文件路径                                               | ./logs/app.log |
| FileConfig.MaxSize    | VIRLOG_FILE_MAX_SIZE     | 单个日志文件最大大小 (MB)                                  | 100            |
| FileConfig.MaxBackups | VIRLOG_FILE_MAX_BACKUPS  | 保留的旧日志文件数                                         | 3              |
| FileConfig.MaxAge     | VIRLOG_FILE_MAX_AGE      | 保留的日志文件天数                                         | 28             |
| FileConfig.Compress   | VIRLOG_FILE_COMPRESS     | 是否压缩旧日志                                             | true           |

## 高级配置

### 配置文件热加载

通过设置 `VIRLOG_CONFFILE` 环境变量，指定配置文件路径：

```bash
export VIRLOG_CONFFILE=/path/to/config.json
```

virlog 会自动监听配置文件变化，一旦文件发生变化，会自动重新加载配置，并更新全局日志器。

### 监听配置变化

如果你想在配置变化时执行自定义操作，可以使用配置监听器：

```go
// 创建配置监听器
configChan := make(chan *config.Config, 1)
config.AddListener(configChan)
defer config.RemoveListener(configChan)

// 监听配置变化
go func() {
    for cfg := range configChan {
        // 配置发生变化，执行自定义操作
        fmt.Printf("配置已更新: %s\n", cfg.Level)
    }
}()
```

## 日志级别

virlog 支持以下日志级别（从低到高）：

- **Debug**: 调试信息，用于开发
- **Info**: 一般信息，用于记录应用状态
- **Warn**: 警告信息，表示可能的问题
- **Error**: 错误信息，表示操作失败
- **DPanic**: 严重错误，在开发模式下会 panic
- **Panic**: 严重错误，会导致 panic
- **Fatal**: 致命错误，会导致程序退出

## 字段构造函数

virlog 提供了多种字段构造函数：

```go
logger.Binary("key", []byte("value"))  // 二进制数据
logger.Bool("key", true)               // 布尔值
logger.String("key", "value")          // 字符串
logger.Int("key", 123)                 // 整数
logger.Int64("key", int64(123))        // 64位整数
logger.Float64("key", 123.45)          // 浮点数
logger.Err(err)                        // 错误
logger.Time("key", time.Now())         // 时间
logger.Duration("key", time.Second)    // 时间间隔
logger.Any("key", someValue)           // 任意类型
```

## 贡献

欢迎提交问题和改进建议！

## 许可证

MIT
