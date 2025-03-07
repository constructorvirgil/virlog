package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/virlog/config"
	"github.com/virlog/logger"
)

func main() {
	// 创建示例配置文件
	configPath := filepath.Join(".", "virlog_config.json")

	// 初始化默认配置
	defaultConfig := config.DefaultConfig()
	defaultConfig.Level = "info"
	defaultConfig.Format = "console"
	defaultConfig.DefaultFields = map[string]interface{}{
		"service": "config-example",
		"version": "1.0.0",
	}

	// 保存到文件
	err := config.SaveToFile(defaultConfig, configPath)
	if err != nil {
		fmt.Printf("保存配置文件失败: %v\n", err)
		return
	}

	defer os.Remove(configPath) // 清理文件

	// 设置环境变量，指定配置文件
	os.Setenv("VIRLOG_CONFFILE", configPath)

	// 创建配置变更监听器
	configChan := make(chan *config.Config, 1)
	config.AddListener(configChan)
	defer config.RemoveListener(configChan)

	// 创建logger
	_, err = logger.NewLogger(config.GetConfig())
	if err != nil {
		fmt.Printf("创建日志器失败: %v\n", err)
		return
	}

	// 启动配置监听协程
	go func() {
		for cfg := range configChan {
			fmt.Println("配置已更新，创建新的logger...")
			newLogger, err := logger.NewLogger(cfg)
			if err != nil {
				fmt.Printf("更新logger失败: %v\n", err)
				continue
			}

			// 更新全局logger
			logger.SetDefault(newLogger)

			// 打印当前配置
			fmt.Printf("当前日志级别: %s, 格式: %s\n", cfg.Level, cfg.Format)
		}
	}()

	// 使用默认logger
	logger.Info("程序启动", logger.String("config_file", configPath))

	// 等待2秒
	time.Sleep(2 * time.Second)

	// 修改配置文件
	fmt.Println("修改配置文件...")
	updatedConfig := config.DefaultConfig()
	updatedConfig.Level = "debug" // 修改日志级别
	updatedConfig.Format = "json" // 修改日志格式
	updatedConfig.DefaultFields = map[string]interface{}{
		"service": "config-example",
		"version": "1.0.0",
		"updated": true,
	}

	// 保存更新后的配置
	err = config.SaveToFile(updatedConfig, configPath)
	if err != nil {
		fmt.Printf("更新配置文件失败: %v\n", err)
		return
	}

	// 等待配置更新生效
	time.Sleep(1 * time.Second)

	// 使用更新后的logger
	logger.Debug("这是一条调试日志，在更新配置前不会显示")
	logger.Info("配置已更新", logger.Bool("success", true))

	// 等待1秒显示结果
	time.Sleep(1 * time.Second)
}
