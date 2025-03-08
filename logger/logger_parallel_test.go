package logger

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/virlog/config"
)

// TestParallelSetDefaultAndLogOperations 测试并发环境下设置默认logger和使用默认logger的操作
func TestParallelSetDefaultAndLogOperations(t *testing.T) {
	// 准备测试数据
	const (
		numSetters     = 10  // 设置默认logger的goroutine数量
		numGetters     = 20  // 获取默认logger的goroutine数量
		numLoggers     = 30  // 使用默认logger记录日志的goroutine数量
		operationsEach = 100 // 每个goroutine的操作次数
	)

	// 创建WaitGroup来协调goroutine的完成
	var wg sync.WaitGroup
	wg.Add(numSetters + numGetters + numLoggers)

	// 运行numSetters个goroutine来设置默认logger
	for i := 0; i < numSetters; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					// 如果发生panic，增加计数器
					// 注意：在实际测试中，我们希望不会有panic发生
					t.Errorf("Panic in setter goroutine %d: %v", id, r)
				}
			}()

			for j := 0; j < operationsEach; j++ {
				// 创建新的logger并设置为默认
				cfg := config.DefaultConfig()
				// 修改一些配置以确保每个logger是不同的
				cfg.Level = "info"
				if j%2 == 0 {
					cfg.Level = "debug"
				}
				logger, err := NewLogger(cfg)
				if err != nil {
					t.Errorf("Failed to create logger in setter %d: %v", id, err)
					return
				}

				// 设置为默认logger
				SetDefault(logger)

				// 短暂等待，增加并发冲突的可能性
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// 运行numGetters个goroutine来获取默认logger
	for i := 0; i < numGetters; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in getter goroutine %d: %v", id, r)
				}
			}()

			for j := 0; j < operationsEach; j++ {
				// 获取默认logger
				logger := DefaultLogger()

				// 简单验证logger不为nil
				assert.NotNil(t, logger, "Default logger should not be nil")

				// 短暂等待，增加并发冲突的可能性
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// 运行numLoggers个goroutine来使用默认logger记录日志
	for i := 0; i < numLoggers; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in logger goroutine %d: %v", id, r)
				}
			}()

			for j := 0; j < operationsEach; j++ {
				// 使用不同的日志级别函数
				switch j % 7 {
				case 0:
					Debug("Debug message from goroutine", Int("goroutine_id", id), Int("operation", j))
				case 1:
					Info("Info message from goroutine", Int("goroutine_id", id), Int("operation", j))
				case 2:
					Warn("Warn message from goroutine", Int("goroutine_id", id), Int("operation", j))
				case 3:
					Error("Error message from goroutine", Int("goroutine_id", id), Int("operation", j))
				case 4:
					// 避免使用DPanic, Panic, Fatal，因为这些可能导致测试提前终止
					With(String("source", "test"), Int("goroutine_id", id)).
						Info("With fields log message", Int("operation", j))
				case 5:
					// 测试SetLevel
					SetLevel(InfoLevel)
				case 6:
					// 测试With后链式调用
					With(String("key1", "value1")).
						With(String("key2", "value2")).
						Info("Chained with log message", Int("goroutine_id", id), Int("operation", j))
				}

				// 短暂等待，增加并发冲突的可能性
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
}

// TestParallelLoggerConcurrentUsage 测试单个logger实例的并发使用
func TestParallelLoggerConcurrentUsage(t *testing.T) {
	// 创建一个logger实例
	logger, err := NewLogger(config.DefaultConfig())
	assert.NoError(t, err, "Failed to create logger")

	// 准备测试数据
	const (
		numGoroutines  = 50  // 并发goroutine数量
		operationsEach = 100 // 每个goroutine的操作次数
	)

	// 创建WaitGroup来协调goroutine的完成
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// 运行多个goroutine并发使用同一个logger实例
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsEach; j++ {
				// 混合使用各种日志操作
				switch j % 6 {
				case 0:
					logger.Info("Direct info message", Int("goroutine_id", id), Int("operation", j))
				case 1:
					logger.Error("Direct error message", Int("goroutine_id", id), Int("operation", j))
				case 2:
					logger.With(String("key", "value"), Int("goroutine_id", id)).
						Info("With method message", Int("operation", j))
				case 3:
					// 创建具有新字段的衍生logger
					newLogger := logger.With(String("context", "derived"), Int("goroutine_id", id))
					newLogger.Info("Derived logger message", Int("operation", j))
				case 4:
					// 修改日志级别再改回来
					originalLevel := InfoLevel // 假设默认是InfoLevel
					logger.SetLevel(DebugLevel)
					logger.Debug("Debug level message should appear now", Int("goroutine_id", id))
					logger.SetLevel(originalLevel)
				case 5:
					// 使用Sync方法
					// 注意：实际环境中，频繁调用Sync可能会影响性能
					_ = logger.Sync()
				}
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
}

// TestRaceDetectionInConfigWatcher 测试配置监听器的并发安全性
func TestRaceDetectionInConfigWatcher(t *testing.T) {
	// 这个测试主要依靠go test -race运行时的竞争检测
	// 同时模拟配置更新和日志记录操作

	// 创建一个配置通道模拟配置变更
	configChan := make(chan *config.Config, 5)

	// 启动一个goroutine发送配置更新
	go func() {
		for i := 0; i < 10; i++ {
			cfg := config.DefaultConfig()
			cfg.Level = "debug"
			if i%2 == 0 {
				cfg.Level = "info"
			}
			configChan <- cfg
			time.Sleep(time.Millisecond * 10)
		}
		close(configChan)
	}()

	// 启动多个goroutine进行日志记录
	var wg sync.WaitGroup
	wg.Add(20)

	for i := 0; i < 20; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 50; j++ {
				Info("Log message during config updates", Int("goroutine_id", id), Int("count", j))
				time.Sleep(time.Millisecond * 5)
			}
		}(i)
	}

	// 模拟配置处理过程
	for cfg := range configChan {
		newLogger, err := NewLogger(cfg)
		if err == nil {
			SetDefault(newLogger)
		}
	}

	// 等待所有日志记录完成
	wg.Wait()
}
