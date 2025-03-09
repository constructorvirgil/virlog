package main

import (
	"github.com/constructorvirgil/virlog/config"
	"github.com/constructorvirgil/virlog/logger"
)

func main() {
	// 使用默认配置
	logger.Info("这是一条信息日志", logger.String("key", "value"))
	logger.Error("这是一条错误日志", logger.Int("code", 500))

	// 创建自定义logger
	cfg := config.DefaultConfig()
	cfg.Level = "debug"
	cfg.Format = "console"
	cfg.DefaultFields = map[string]interface{}{
		"service": "example-service",
	}

	log, err := logger.NewLogger(cfg)
	if err != nil {
		logger.Fatal("创建日志器失败", logger.Err(err))
	}
	defer log.Sync()

	// 使用自定义logger
	log.Debug("这是一条调试日志")
	log.Info("这是一条信息日志", logger.String("custom", "value"))

	// 带字段的logger
	userLogger := log.With(
		logger.String("user_id", "12345"),
		logger.String("username", "test_user"),
	)
	userLogger.Info("用户登录成功")
	userLogger.Warn("用户尝试访问受限资源", logger.String("resource", "/admin"))

	// 动态修改日志级别
	log.SetLevel(logger.WarnLevel)
	log.Debug("此消息不会显示") // 因为级别已提高到Warn
	log.Warn("此警告消息会显示")
}
